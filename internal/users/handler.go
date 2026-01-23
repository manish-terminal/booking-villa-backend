package users

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/db"
)

// PropertyLister is a function type to list properties by owner (avoids import cycle).
type PropertyLister func(ctx context.Context, ownerPhone string) ([]string, error)

// Handler provides HTTP handlers for user/agent endpoints.
type Handler struct {
	service        *Service
	listProperties PropertyLister
}

// NewHandler creates a new user handler.
// propertyLister should return a list of property IDs for a given owner phone.
func NewHandler(dbClient *db.Client, propertyLister PropertyLister) *Handler {
	return &Handler{
		service:        NewService(dbClient),
		listProperties: propertyLister,
	}
}

// getClaimsFromRequest extracts user claims from request headers (set by auth middleware).
func getClaimsFromRequest(request events.APIGatewayProxyRequest) (phone, role string, ok bool) {
	phone = request.Headers["X-User-Phone"]
	if phone == "" {
		phone = request.Headers["x-user-phone"]
	}
	role = request.Headers["X-User-Role"]
	if role == "" {
		role = request.Headers["x-user-role"]
	}
	ok = phone != ""
	return
}

// APIResponse creates a standardized API Gateway response.
func APIResponse(statusCode int, body interface{}) events.APIGatewayProxyResponse {
	jsonBody, _ := json.Marshal(body)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Headers": "Content-Type,Authorization",
		},
		Body: string(jsonBody),
	}
}

// ErrorResponse creates a standardized error response.
func ErrorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	return APIResponse(statusCode, map[string]string{"error": message})
}

// HandleListAgents handles the GET /agents endpoint.
// Owners see only agents linked to their properties. Admins see all.
func (h *Handler) HandleListAgents(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	phone, role, ok := getClaimsFromRequest(request)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	var ownerPropertyIDs []string

	// If not admin, get the owner's properties to filter agents
	if role != "admin" {
		owner, err := h.service.GetUserByPhone(ctx, phone)
		if err != nil {
			return ErrorResponse(http.StatusInternalServerError, "Failed to get user"), nil
		}
		if owner == nil {
			return ErrorResponse(http.StatusNotFound, "User not found"), nil
		}

		// Get properties owned by this user
		props, err := h.listProperties(ctx, phone)
		if err != nil {
			return ErrorResponse(http.StatusInternalServerError, "Failed to get properties"), nil
		}

		ownerPropertyIDs = props
	}

	agents, err := h.service.ListAgentsForOwner(ctx, ownerPropertyIDs)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to list agents"), nil
	}

	// Convert to response format
	responses := make([]UserResponse, len(agents))
	for i, agent := range agents {
		responses[i] = agent.ToResponse()
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"agents": responses,
		"count":  len(responses),
	}), nil
}

// UpdateAgentStatusRequest represents the request body for updating agent status.
type UpdateAgentStatusRequest struct {
	Active bool `json:"active"`
}

// HandleUpdateAgentStatus handles the PATCH /agents/{phone}/status endpoint.
func (h *Handler) HandleUpdateAgentStatus(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	agentPhone := request.PathParameters["phone"]
	if agentPhone == "" {
		return ErrorResponse(http.StatusBadRequest, "Agent phone is required"), nil
	}

	// URL decode the phone (in case it has special chars like +)
	agentPhone = strings.ReplaceAll(agentPhone, "%2B", "+")

	phone, role, ok := getClaimsFromRequest(request)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	var req UpdateAgentStatusRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	// Verify agent exists and is actually an agent
	agent, err := h.service.GetUserByPhone(ctx, agentPhone)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get agent"), nil
	}
	if agent == nil {
		return ErrorResponse(http.StatusNotFound, "Agent not found"), nil
	}
	if agent.Role != RoleAgent {
		return ErrorResponse(http.StatusBadRequest, "User is not an agent"), nil
	}

	// Permission check: Owner can only manage agents linked to their properties
	if role != "admin" {
		// Get owner's properties
		props, err := h.listProperties(ctx, phone)
		if err != nil {
			return ErrorResponse(http.StatusInternalServerError, "Failed to get properties"), nil
		}

		// Check if agent has any overlap with owner's properties
		ownerPropSet := make(map[string]bool)
		for _, pid := range props {
			ownerPropSet[pid] = true
		}

		hasOverlap := false
		for _, agentProp := range agent.ManagedProperties {
			if ownerPropSet[agentProp] {
				hasOverlap = true
				break
			}
		}

		if !hasOverlap {
			return ErrorResponse(http.StatusForbidden, "You do not have permission to manage this agent"), nil
		}
	}

	// Update agent status
	if err := h.service.SetAgentActive(ctx, agentPhone, req.Active, phone); err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to update agent status"), nil
	}

	// Get updated agent
	updatedAgent, _ := h.service.GetUserByPhone(ctx, agentPhone)

	return APIResponse(http.StatusOK, map[string]interface{}{
		"agent":   updatedAgent.ToResponse(),
		"message": "Agent status updated successfully",
	}), nil
}
