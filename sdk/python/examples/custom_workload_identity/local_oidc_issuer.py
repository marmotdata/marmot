#!/usr/bin/env python3
"""A throwaway OIDC issuer, to exercise Marmot's RFC 8693 token exchange locally.

Serves for discovery at /.well-known/openid-configuration.
Signed with an in-memory RSA key and writes it to `--token-file`, where the
`FileSubjectTokenSource` in custom_workload_identity.py picks it up.

    python examples/custom_workload_identity/local_oidc_issuer.py \
        --audience marmot-workload \
        --email dev@example.com --token-file /tmp/marmot-subject.jwt

Configure Marmot to trust it (config.yaml), then start the server:

    auth:
      generic_oidc:
        enabled: true
        type: generic_oidc
        name: Local Dev
        url: http://localhost:9001
        client_id: marmot-workload
        allowed_audiences: [marmot-workload]

The issuer URL must match `url` exactly: go-oidc compares the `iss` claim with
the discovery document's issuer. See examples/README.md for the full recipe.
"""

from __future__ import annotations

import argparse
import base64
import json
import pathlib
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import jwt
from cryptography.hazmat.primitives.asymmetric import rsa

KEY_ID = "dev-key-1"


def _b64(value: int) -> str:
    raw = value.to_bytes((value.bit_length() + 7) // 8, "big")
    return base64.urlsafe_b64encode(raw).decode().rstrip("=")


class Issuer:
    def __init__(self, url: str, audience: str, email: str, subject: str) -> None:
        self.url = url.rstrip("/")
        self.audience = audience
        self.email = email
        self.subject = subject
        self.key = rsa.generate_private_key(public_exponent=65537, key_size=2048)

    @property
    def discovery(self) -> dict[str, object]:
        return {
            "issuer": self.url,
            "jwks_uri": f"{self.url}/jwks.json",
            "authorization_endpoint": f"{self.url}/authorize",
            "token_endpoint": f"{self.url}/token",
            "userinfo_endpoint": f"{self.url}/userinfo",
            "response_types_supported": ["id_token"],
            "subject_types_supported": ["public"],
            "id_token_signing_alg_values_supported": ["RS256"],
        }

    @property
    def jwks(self) -> dict[str, object]:
        numbers = self.key.public_key().public_numbers()
        return {
            "keys": [
                {
                    "kty": "RSA",
                    "use": "sig",
                    "alg": "RS256",
                    "kid": KEY_ID,
                    "n": _b64(numbers.n),
                    "e": _b64(numbers.e),
                }
            ]
        }

    def mint(self, ttl: int = 3600) -> str:
        now = int(time.time())
        return jwt.encode(
            {
                "iss": self.url,
                "sub": self.subject,
                "aud": self.audience,
                "email": self.email,
                "name": "Local Dev Workload",
                "iat": now,
                "exp": now + ttl,
            },
            self.key,
            algorithm="RS256",
            headers={"kid": KEY_ID},
        )


def serve(issuer: Issuer, port: int) -> None:
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            routes = {
                "/.well-known/openid-configuration": issuer.discovery,
                "/jwks.json": issuer.jwks,
            }
            body = routes.get(self.path)
            if body is None:
                self.send_error(404)
                return
            payload = json.dumps(body).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, fmt: str, *args: object) -> None:
            print(f"issuer: {fmt % args}")

    ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--port", type=int, default=9001)
    parser.add_argument("--audience", default="marmot-workload")
    parser.add_argument("--email", default="dev@example.com")
    parser.add_argument("--subject", default="local-dev-workload")
    parser.add_argument("--token-file", default="/tmp/marmot-subject.jwt")  # noqa: S108
    args = parser.parse_args()

    issuer = Issuer(f"http://localhost:{args.port}", args.audience, args.email, args.subject)
    token_file = pathlib.Path(args.token_file)
    token_file.write_text(issuer.mint())

    print(f"issuer:     {issuer.url}")
    print(f"discovery:  {issuer.url}/.well-known/openid-configuration")
    print(f"audience:   {args.audience}")
    print(f"subject:    {args.subject} ({args.email})")
    print(f"token file: {token_file}")
    serve(issuer, args.port)


if __name__ == "__main__":
    main()
