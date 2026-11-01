package sftp

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// dialTimeout bounds both the TCP connect and the SSH handshake, so an
// unreachable server fails the run instead of hanging it.
const dialTimeout = 30 * time.Second

// fileSystem is the slice of the SFTP protocol this plugin uses. It is
// an interface so the walk and the file readers can be tested against a
// directory on disk instead of a live server.
type fileSystem interface {
	// ReadDir lists one directory. The entries describe the links
	// themselves, not their targets, so a symlink can be recognised.
	ReadDir(dir string) ([]os.FileInfo, error)
	// Stat follows symlinks, and is only used to learn what one points at.
	Stat(path string) (os.FileInfo, error)
	// RealPath resolves a path to its canonical form, which is how the
	// walk notices it has already been somewhere.
	RealPath(path string) (string, error)
	// Open opens a file for reading.
	Open(path string) (io.ReadCloser, error)
}

// connection is a live SFTP session plus the SSH transport under it.
type connection struct {
	client *sftp.Client
	ssh    *ssh.Client
	conn   net.Conn
}

func (c *connection) ReadDir(dir string) ([]os.FileInfo, error) { return c.client.ReadDir(dir) }
func (c *connection) Stat(path string) (os.FileInfo, error)     { return c.client.Stat(path) }
func (c *connection) RealPath(path string) (string, error)      { return c.client.RealPath(path) }

func (c *connection) Open(path string) (io.ReadCloser, error) { return c.client.Open(path) }

func (c *connection) Close() {
	if c.client != nil {
		c.client.Close()
	}
	if c.ssh != nil {
		c.ssh.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// connect opens an SSH connection and starts an SFTP session on it.
func connect(ctx context.Context, config *Config) (*connection, error) {
	auths, err := authMethods(config)
	if err != nil {
		return nil, err
	}

	callback, err := hostKeyCallback(config)
	if err != nil {
		return nil, err
	}

	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))

	// ssh.Dial has no context, so the TCP connection is made separately
	// and handed to the SSH handshake.
	conn, err := (&net.Dialer{Timeout: dialTimeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dialling: %w", err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(dialTimeout))
	}

	sshConn, channels, requests, err := ssh.NewClientConn(conn, address, &ssh.ClientConfig{
		User:            config.Username,
		Auth:            auths,
		HostKeyCallback: callback,
		Timeout:         dialTimeout,
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}

	// The handshake deadline must not outlive the handshake, or a long
	// walk would be cut off mid-run.
	_ = conn.SetDeadline(time.Time{})

	sshClient := ssh.NewClient(sshConn, channels, requests)

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		conn.Close()
		return nil, fmt.Errorf("starting sftp session: %w", err)
	}

	return &connection{client: client, ssh: sshClient, conn: conn}, nil
}

// authMethods builds the login methods from the config. A password and a
// key can both be configured; the server picks.
func authMethods(config *Config) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if key := strings.TrimSpace(config.PrivateKey); key != "" {
		signer, err := parsePrivateKey(key, config.PrivateKeyPassphrase)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if config.Password != "" {
		methods = append(methods, ssh.Password(config.Password))
		// Some servers ask for the password through keyboard-interactive
		// instead of the password method.
		methods = append(methods, ssh.KeyboardInteractive(
			func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = config.Password
				}
				return answers, nil
			}))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("either password or private_key is required")
	}
	return methods, nil
}

// supportedKeyTypes are the private key algorithms this plugin accepts,
// named the way ssh-keygen names them so an error message is actionable.
var supportedKeyTypes = map[string]bool{
	ssh.KeyAlgoRSA:      true,
	ssh.KeyAlgoED25519:  true,
	ssh.KeyAlgoECDSA256: true,
	ssh.KeyAlgoECDSA384: true,
	ssh.KeyAlgoECDSA521: true,
}

func parsePrivateKey(key, passphrase string) (ssh.Signer, error) {
	var (
		signer ssh.Signer
		err    error
	)

	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(key))
	}
	if err != nil {
		// The key itself is never echoed back.
		if _, missing := err.(*ssh.PassphraseMissingError); missing {
			return nil, fmt.Errorf("private_key is encrypted, set private_key_passphrase")
		}
		return nil, fmt.Errorf("parsing private_key: %w", err)
	}

	if keyType := signer.PublicKey().Type(); !supportedKeyTypes[keyType] {
		return nil, fmt.Errorf("private_key type %q is not supported, use an RSA, Ed25519 or ECDSA key", keyType)
	}

	return signer, nil
}

// hostKeyCallback pins the server's key when one is configured. Without
// one there is nothing to compare against, so any key is accepted.
func hostKeyCallback(config *Config) (ssh.HostKeyCallback, error) {
	text := strings.TrimSpace(config.HostKey)
	if text == "" {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	key, err := parseHostKey(text)
	if err != nil {
		return nil, err
	}
	return ssh.FixedHostKey(key), nil
}

// parseHostKey accepts a key in authorized_keys form ("ssh-ed25519 AAAA...")
// and in known_hosts form, which is the same line with the host in front.
func parseHostKey(text string) (ssh.PublicKey, error) {
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(text))
	if err == nil {
		return key, nil
	}

	if _, rest, found := strings.Cut(text, " "); found {
		if key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(rest)); err == nil {
			return key, nil
		}
	}

	return nil, fmt.Errorf("parsing host_key: %w", err)
}

// ownerIDs reads the uid and gid an SFTP server reported for an entry.
// Other filesystems do not carry them, so the caller is told when they
// are absent rather than shown a zero.
func ownerIDs(info os.FileInfo) (uid, gid uint32, ok bool) {
	stat, isSFTP := info.Sys().(*sftp.FileStat)
	if !isSFTP {
		return 0, 0, false
	}
	return stat.UID, stat.GID, true
}
