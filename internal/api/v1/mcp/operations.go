package mcp

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// The same handler also answers GET (the SSE stream) and OPTIONS (CORS
// preflight) on this path; only POST is documented, since a JSON-RPC request is
// the operation a client makes.
//
// @Summary MCP endpoint
// @Description JSON-RPC 2.0 over the MCP Streamable HTTP transport. Requires assets:view, glossary:view and teams:view.
// @Tags mcp
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "JSON-RPC 2.0 request"
// @Success 200 {object} map[string]interface{} "JSON-RPC 2.0 response"
// @Failure 401 {object} common.ErrorResponse
// @ID postMcp
// @Router /api/v1/mcp [post]
func (h *Handler) handleMCP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id, Authorization, X-API-Key")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Extract authenticated user from context
	user, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		log.Warn().Msg("MCP request without authenticated user")
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	log.Debug().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("user_id", user.ID).
		Str("username", user.Username).
		Str("content-type", r.Header.Get("Content-Type")).
		Str("accept", r.Header.Get("Accept")).
		Str("mcp-session-id", r.Header.Get("Mcp-Session-Id")).
		Msg("MCP request received")

	mcpHandler := mcpsdk.NewStreamableHTTPHandler(
		func(req *http.Request) *mcpsdk.Server {
			server := h.mcpServer.CreateMCPServer(req.Context(), user)
			log.Debug().
				Str("user_id", user.ID).
				Str("username", user.Username).
				Msg("MCP server instance created for user")
			return server
		},
		&mcpsdk.StreamableHTTPOptions{
			Stateless: true,
		},
	)

	mcpHandler.ServeHTTP(w, r)
	log.Debug().Msg("MCP request handled")
}
