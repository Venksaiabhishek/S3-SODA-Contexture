package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tmc/langchaingo/llms"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== Interface defines ==========

// RequestProcessorInterface defines core request processing functionality
//
// Available Functions:
//   - ProcessRequest()                : Process natural language infrastructure requests
//   - gatherDecisionContext()         : Gather context for decision-making
//   - generateDecisionWithPlan()      : Generate AI decision with detailed execution plan
//   - validateDecision()              : Validate agent decisions for safety and consistency
//   - buildDecisionWithPlanPrompt()   : Build comprehensive prompts for AI decision making
//   - parseAIResponseWithPlan()       : Parse AI responses into structured execution plans
//
// This file handles the core request processing pipeline from natural language
// input to validated execution plans ready for infrastructure operations.
//
// Usage Example:
//   1. decision, err := agent.ProcessRequest(ctx, "Create a web server with load balancer")
//   2. // Decision contains validated execution plan ready for deployment

// ProcessRequest processes a natural language infrastructure request and generates a plan
func (a *StateAwareAgent) ProcessRequest(ctx context.Context, request string) (*types.AgentDecision, error) {
	a.Logger.WithField("request", request).Info("Processing infrastructure request")

	// Create decision ID
	decisionID := uuid.New().String()

	// Gather context
	decisionContext, err := a.gatherDecisionContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to gather decision context: %w", err)
	}

	// Generate AI decision with detailed execution plan
	decision, err := a.generateDecisionWithPlan(ctx, decisionID, request, decisionContext)
	if err != nil {
		return nil, fmt.Errorf("failed to generate decision: %w", err)
	}

	// Validate decision
	if err := a.validateDecision(decision, decisionContext); err != nil {
		return nil, fmt.Errorf("decision validation failed: %w", err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"decision_id": decision.ID,
		"action":      decision.Action,
		"confidence":  decision.Confidence,
		"plan_steps":  len(decision.ExecutionPlan),
	}).Info("Infrastructure request processed successfully")

	return decision, nil
}

// gatherDecisionContext gathers context for decision-making
func (a *StateAwareAgent) gatherDecisionContext(ctx context.Context, request string) (*DecisionContext, error) {
	a.Logger.Debug("Gathering decision context")

	// Use MCP server to analyze infrastructure state
	currentState, discoveredResources, _, err := a.AnalyzeInfrastructureState(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze infrastructure state: %w", err)
	}

	// Use MCP server to detect conflicts
	conflicts, err := a.DetectInfrastructureConflicts(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to detect conflicts: %w", err)
	}

	// Use MCP server to get deployment order
	deploymentOrder, _, err := a.PlanInfrastructureDeployment(ctx, nil, false)
	if err != nil {
		// Non-fatal error - continue without deployment order
		a.Logger.WithError(err).Warn("Failed to calculate deployment order")
		deploymentOrder = []string{}
	}

	// Analyze resource correlation for better decision making
	// resourceCorrelation := a.analyzeResourceCorrelation(currentState, discoveredResources)

	return &DecisionContext{
		Request:             request,
		CurrentState:        currentState,
		DiscoveredState:     discoveredResources,
		Conflicts:           conflicts,
		DependencyGraph:     nil, // Will be handled by MCP server
		DeploymentOrder:     deploymentOrder,
		ResourceCorrelation: nil,
	}, nil
}

// generateDecisionWithPlan uses AI to generate a decision with detailed execution plan
func (a *StateAwareAgent) generateDecisionWithPlan(ctx context.Context, decisionID, request string, context *DecisionContext) (*types.AgentDecision, error) {
	a.Logger.Debug("Generating AI decision with execution plan")

	// Create prompt for the AI that includes plan generation
	prompt, promptErr := a.buildDecisionWithPlanPrompt(request, context)
	if promptErr != nil {
		return nil, fmt.Errorf("failed to build decision prompt: %w", promptErr)
	}

	// Log prompt details for debugging
	a.Logger.WithFields(map[string]interface{}{
		"prompt_length": len(prompt),
		"max_tokens":    a.config.MaxTokens,
		"temperature":   a.config.Temperature,
		"provider":      a.config.Provider,
		"model":         a.config.Model,
	}).Info("Calling LLM with prompt")

	var response string
	var err error

	// Check if using Amazon Nova model
	if strings.Contains(a.config.Model, "amazon.nova") {
		messages := []llms.MessageContent{
			{
				Role: llms.ChatMessageTypeSystem,
				Parts: []llms.ContentPart{
					llms.TextContent{Text: "You are an expert AWS infrastructure automation agent with comprehensive state management capabilities. You must respond with valid JSON only."},
				},
			},
			{
				Role: llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{
					llms.TextContent{Text: prompt},
				},
			},
		}

		// Generate response using GenerateContent for Nova
		resp, err := a.llm.GenerateContent(ctx, messages,
			llms.WithTemperature(a.config.Temperature),
			llms.WithMaxTokens(a.config.MaxTokens))

		if err != nil {
			return nil, fmt.Errorf("failed to generate AI response with Nova: %w", err)
		}

		// Validate and extract response from Nova
		if len(resp.Choices) < 1 {
			return nil, fmt.Errorf("nova returned empty response - no choices available")
		}

		response = resp.Choices[0].Content

	} else {
		// For non-Nova models, use the original GenerateFromSinglePrompt
		response, err = llms.GenerateFromSinglePrompt(ctx, a.llm, prompt,
			llms.WithTemperature(a.config.Temperature),
			llms.WithMaxTokens(a.config.MaxTokens))

		if err != nil {
			return nil, fmt.Errorf("failed to generate AI response: %w", err)
		}
	}

	if a.config.EnableDebug {
		// Comprehensive response logging
		a.Logger.WithFields(map[string]interface{}{
			"response_length":  len(response),
			"response_empty":   len(response) == 0,
			"response_content": response, // Log full response for debugging
		}).Info("LLM Response received")
	}

	// Handle empty response immediately
	if len(response) == 0 {
		a.Logger.Error("LLM returned empty response - check API key, model availability, and prompt")
		return nil, fmt.Errorf("LLM returned empty response - possible API key, model, or prompt issue")
	}

	if a.config.EnableDebug {
		// Log response characteristics for debugging
		a.Logger.WithFields(map[string]interface{}{
			"response_length":     len(response),
			"max_tokens_config":   a.config.MaxTokens,
			"starts_with_brace":   strings.HasPrefix(response, "{"),
			"ends_with_brace":     strings.HasSuffix(response, "}"),
			"probable_truncation": strings.HasPrefix(response, "{") && !strings.HasSuffix(response, "}"),
		}).Debug("LLM Response Analysis")
	}

	// Strip chain-of-thought reasoning if the model output thinking tokens before JSON
	// Some Gemini model variants (e.g., flash with thinking enabled) output reasoning
	// text like "Wait, I'll check the action field..." before the actual JSON.
	response = a.stripChainOfThought(response)

	// Check for potential token limit issues explicitly before parsing
	isTruncated := len(response) > 0 && strings.HasPrefix(strings.TrimSpace(response), "{") && !strings.HasSuffix(strings.TrimSpace(response), "}")
	if isTruncated {
		a.Logger.WithFields(map[string]interface{}{
			"response_length": len(response),
			"max_tokens":      a.config.MaxTokens,
			"last_100_chars":  response[max(0, len(response)-100):],
		}).Warn("Response appears truncated - this will likely cause a validation failure")
	}

	// Parse the AI response with execution plan
	decision, err := a.parseAIResponseWithPlan(decisionID, request, response)
	if err != nil {
		if isTruncated {
			return nil, fmt.Errorf("AI response was truncated due to token limits or safety filters, and could not be recovered: %w", err)
		}
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}

// validateDecision validates an agent decision
func (a *StateAwareAgent) validateDecision(decision *types.AgentDecision, context *DecisionContext) error {
	a.Logger.Debug("Validating agent decision")

	// Validate action
	validActions := map[string]bool{
		"create_infrastructure": true,
		"update_infrastructure": true,
		"delete_infrastructure": true,
		"resolve_conflicts":     true,
		"no_action":             true,
	}

	if !validActions[decision.Action] {
		return fmt.Errorf("invalid action: %s", decision.Action)
	}

	planValidActions := map[string]bool{
		"create": true,
		"query":  true,
		"modify": true,
		// "validate": true,
		// "delete":   true,
	}

	// Get available MCP tools for validation
	a.capabilityMutex.RLock()
	availableTools := make(map[string]bool)
	for toolName := range a.mcpTools {
		availableTools[toolName] = true
	}
	a.capabilityMutex.RUnlock()

	// Validate each plan step
	for i, planStep := range decision.ExecutionPlan {
		// Validate action type
		if !planValidActions[planStep.Action] {
			return fmt.Errorf("invalid plan action '%s' in step %d (%s)", planStep.Action, i+1, planStep.ID)
		}

		// Validate MCP tool exists if specified
		if planStep.MCPTool != "" {
			if !availableTools[planStep.MCPTool] {
				// Get list of available tools for error message
				availableToolsList := make([]string, 0, len(availableTools))
				for tool := range availableTools {
					availableToolsList = append(availableToolsList, tool)
				}

				return fmt.Errorf(
					"invalid MCP tool '%s' in step %d (%s): tool not found in discovered capabilities. Available tools: %v",
					planStep.MCPTool,
					i+1,
					planStep.ID,
					availableToolsList,
				)
			}
		}
	}

	// Check for critical conflicts if auto-resolve is disabled
	if !a.config.AutoResolveConflicts && len(context.Conflicts) > 0 {
		for _, conflict := range context.Conflicts {
			if conflict.ConflictType == "dependency" {
				return fmt.Errorf("critical dependency conflict detected, manual resolution required")
			}
		}
	}

	return nil
}

// buildDecisionWithPlanPrompt builds a prompt for AI decision-making with execution plan
func (a *StateAwareAgent) buildDecisionWithPlanPrompt(request string, context *DecisionContext) (string, error) {
	var prompt strings.Builder

	prompt.WriteString("You are an expert AWS infrastructure automation agent with comprehensive state management capabilities.\n\n")

	// Add available tools context
	toolsContext, err := a.getAvailableToolsContext()
	if err != nil {
		return "", fmt.Errorf("failed to get available tools context: %w", err)
	}
	prompt.WriteString(toolsContext)
	prompt.WriteString("\n")

	prompt.WriteString("USER REQUEST: " + request + "\n\n")

	// === INFRASTRUCTURE STATE OVERVIEW ===
	prompt.WriteString("📊 INFRASTRUCTURE STATE OVERVIEW:\n")
	prompt.WriteString("Analyze ALL available resources from the state file to make informed decisions.\n\n")

	// Show current managed resources from state file - capped at 5 for prompt efficiency
	// SMART FILTERING: only show resources that might be relevant to the query keywords
	const maxResourcesInPrompt = 5
	queryLower := strings.ToLower(request)
	if len(context.CurrentState.Resources) > 0 {
		prompt.WriteString("🏗️ RELEVANT MANAGED RESOURCES (filtered for efficiency):\n")
		count := 0
		for resourceID, resource := range context.CurrentState.Resources {
			if count >= maxResourcesInPrompt {
				break
			}

			// Only include if ID or Type matches query keywords, or if we have space
			resIDLower := strings.ToLower(resourceID)
			resTypeLower := strings.ToLower(resource.Type)
			isRelevant := strings.Contains(queryLower, resIDLower) || 
						  strings.Contains(resIDLower, queryLower) ||
						  strings.Contains(queryLower, resTypeLower)

			if isRelevant || count < 3 { // Always show some resources if we have space
				count++
				prompt.WriteString(fmt.Sprintf("- %s (%s): %s", resourceID, resource.Type, resource.Status))

				// Extract and show minimal properties from state file
				if resource.Properties != nil {
					var properties []string
					propCount := 0

					// Extract from direct properties
					for key, value := range resource.Properties {
						if propCount >= 2 { // Extremely limited properties
							break
						}
						
						if key == "mcp_response" {
							// Extract from nested mcp_response
							if mcpMap, ok := value.(map[string]interface{}); ok {
								for mcpKey, mcpValue := range mcpMap {
									if propCount >= 2 {
										break
									}
									if mcpKey != "success" && mcpKey != "timestamp" && mcpKey != "message" {
										valStr := fmt.Sprintf("%v", mcpValue)
										if len(valStr) > 40 {
											valStr = valStr[:40] + "..."
										}
										properties = append(properties, fmt.Sprintf("%s:%s", mcpKey, valStr))
										propCount++
									}
								}
							}
						} else if key != "status" {
							valStr := fmt.Sprintf("%v", value)
							if len(valStr) > 40 {
								valStr = valStr[:40] + "..."
							}
							properties = append(properties, fmt.Sprintf("%s:%s", key, valStr))
							propCount++
						}
					}

					if len(properties) > 0 {
						prompt.WriteString(fmt.Sprintf(" [%s]", strings.Join(properties, ", ")))
					}
				}
				prompt.WriteString("\n")
			}
		}
		if count >= maxResourcesInPrompt {
			prompt.WriteString("- ... (additional resources omitted for efficiency)\n")
		}
		prompt.WriteString("\n")
	}

	// Show discovered AWS resources (not in state file)
	if len(context.DiscoveredState) > 0 {
		prompt.WriteString("🔍 DISCOVERED AWS RESOURCES (not managed in state file):\n")
		for _, resource := range context.DiscoveredState {
			prompt.WriteString(fmt.Sprintf("- %s (%s): %s", resource.ID, resource.Type, resource.Status))

			if resource.Properties != nil {
				var properties []string

				// Show most relevant properties for each resource type
				relevantKeys := []string{"vpcId", "groupName", "instanceType", "cidrBlock", "name", "state", "availabilityZone"}
				for _, key := range relevantKeys {
					if value, exists := resource.Properties[key]; exists {
						properties = append(properties, fmt.Sprintf("%s:%v", key, value))
					}
				}

				if len(properties) > 0 {
					prompt.WriteString(fmt.Sprintf(" [%s]", strings.Join(properties, ", ")))
				}
			}
			prompt.WriteString("\n")
		}
		prompt.WriteString("\n")
	}

	// Show resource correlations if any
	// if len(context.ResourceCorrelation) > 0 {
	// 	prompt.WriteString("🔗 RESOURCE CORRELATIONS:\n")
	// 	for managedID, correlation := range context.ResourceCorrelation {
	// 		prompt.WriteString(fmt.Sprintf("- State file resource '%s' correlates with AWS resource '%s' (confidence: %.2f)\n",
	// 			managedID, correlation.DiscoveredResource.ID, correlation.MatchConfidence))
	// 	}
	// 	prompt.WriteString("\n")
	// }

	// Show any conflicts
	if len(context.Conflicts) > 0 {
		prompt.WriteString("⚠️ DETECTED CONFLICTS:\n")
		for _, conflict := range context.Conflicts {
			prompt.WriteString(fmt.Sprintf("- %s: %s (Resource: %s)\n", conflict.ConflictType, conflict.Details, conflict.ResourceID))
		}
		prompt.WriteString("\n")
	}

	// Load decision guidelines from template file
	decisionTemplate, err := a.loadTemplate("settings/templates/decision-plan-prompt-optimized.txt")
	if err != nil {
		a.Logger.WithError(err).Error("Failed to load decision template")
		return "", fmt.Errorf("failed to load decision template: %w", err)
	}
	prompt.WriteString(decisionTemplate)

	return prompt.String(), nil
}

// parseAIResponseWithPlan parses the AI response into an AgentDecision with execution plan
func (a *StateAwareAgent) parseAIResponseWithPlan(decisionID, request, response string) (*types.AgentDecision, error) {
	a.Logger.Debug("Parsing AI response for execution plan")

	// Check if response appears to be truncated JSON
	if strings.HasPrefix(response, "{") && !strings.HasSuffix(response, "}") && a.config.EnableDebug {
		a.Logger.WithFields(map[string]interface{}{
			"response_starts_with": response[:min(100, len(response))],
			"response_ends_with":   response[max(0, len(response)-100):],
		}).Warn("Response appears to be truncated JSON")
	}

	// Try multiple JSON extraction methods
	jsonStr := a.extractJSON(response)
	if jsonStr == "" {
		// Try alternative extraction methods
		jsonStr = a.extractJSONAlternative(response)
	}

	// Special handling for potentially truncated responses
	if jsonStr == "" && strings.HasPrefix(response, "{") {
		a.Logger.Warn("Attempting to parse potentially truncated JSON response")
		jsonStr = a.attemptTruncatedJSONParse(response)
	}

	if jsonStr == "" {
		if a.config.EnableDebug {
			a.Logger.WithFields(map[string]interface{}{
				"response_preview":  response[:min(500, len(response))],
				"response_length":   len(response),
				"starts_with_brace": strings.HasPrefix(response, "{"),
				"ends_with_brace":   strings.HasSuffix(response, "}"),
			}).Error("No valid JSON found in AI response")
		}
		return nil, fmt.Errorf("no valid JSON found in AI response")
	}

	if a.config.EnableDebug {
		a.Logger.WithFields(map[string]interface{}{
			"extracted_json_length": len(jsonStr),
			"extracted_json":        jsonStr,
		}).Info("Successfully extracted JSON from AI response")
	}

	// Clean JSON comments that AI models sometimes include
	jsonStr = a.cleanJSONComments(jsonStr)

	// Parse JSON with execution plan - updated for native MCP tool support
	var parsed struct {
		Action        string                 `json:"action"`
		Reasoning     string                 `json:"reasoning"`
		Confidence    float64                `json:"confidence"`
		Parameters    map[string]interface{} `json:"parameters"`
		ExecutionPlan []struct {
			ID                string                 `json:"id"`
			Name              string                 `json:"name"`
			Description       string                 `json:"description"`
			Action            string                 `json:"action"`
			ResourceID        string                 `json:"resourceId"`
			MCPTool           string                 `json:"mcpTool"`        // New: Direct MCP tool name
			ToolParameters    map[string]interface{} `json:"toolParameters"` // New: Direct tool parameters
			Parameters        map[string]interface{} `json:"parameters"`     // Legacy fallback
			DependsOn         []string               `json:"dependsOn"`
			EstimatedDuration string                 `json:"estimatedDuration"`
			Status            string                 `json:"status"`
		} `json:"executionPlan"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		a.Logger.WithError(err).WithField("json", jsonStr).Error("Failed to parse AI response JSON")

		return nil, fmt.Errorf("failed to parse AI response JSON: %w", err)
	}

	// Convert execution plan with native MCP support
	var executionPlan []*types.ExecutionPlanStep
	for _, step := range parsed.ExecutionPlan {
		planStep := &types.ExecutionPlanStep{
			ID:                step.ID,
			Name:              step.Name,
			Description:       step.Description,
			Action:            step.Action,
			ResourceID:        step.ResourceID,
			MCPTool:           step.MCPTool,
			ToolParameters:    step.ToolParameters,
			Parameters:        step.Parameters,
			DependsOn:         step.DependsOn,
			EstimatedDuration: step.EstimatedDuration,
			Status:            step.Status,
		}

		// Ensure we have parameters - use ToolParameters if available, otherwise Parameters
		if len(planStep.ToolParameters) > 0 {
			// Native MCP mode - use ToolParameters as primary
			if planStep.Parameters == nil {
				planStep.Parameters = make(map[string]interface{})
			}
			// Copy tool parameters to legacy parameters for compatibility
			for key, value := range planStep.ToolParameters {
				planStep.Parameters[key] = value
			}
		}

		executionPlan = append(executionPlan, planStep)
	}

	return &types.AgentDecision{
		ID:            decisionID,
		Action:        parsed.Action,
		Resource:      request,
		Reasoning:     parsed.Reasoning,
		Confidence:    parsed.Confidence,
		Parameters:    parsed.Parameters,
		ExecutionPlan: executionPlan,
		Timestamp:     time.Now(),
	}, nil
}
