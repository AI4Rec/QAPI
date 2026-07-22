package service

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type CPAAuthMetadata struct {
	AccountID string
	PlanType  string
	Email     string
}

func ExtractCPAAuthMetadata(values map[string]any) CPAAuthMetadata {
	metadata := CPAAuthMetadata{
		AccountID: firstCPAString(values, "chatgpt_account_id", "account_id"),
		PlanType:  firstCPAString(values, "chatgpt_plan_type", "plan_type"),
		Email:     firstCPAString(values, "email"),
	}
	if accessToken, ok := values["access_token"].(string); ok {
		metadata = mergeCPAMetadata(metadata, metadataFromCPAJWT(accessToken))
	}
	switch idToken := values["id_token"].(type) {
	case map[string]any:
		metadata = mergeCPAMetadata(metadata, metadataFromCPAClaims(idToken))
	case string:
		metadata = mergeCPAMetadata(metadata, metadataFromCPAJWT(idToken))
	}
	return metadata
}

func ResolveAndRepairCPAAuthMetadata(ctx context.Context, file map[string]any) (CPAAuthMetadata, bool, error) {
	metadata := ExtractCPAAuthMetadata(file)
	needsDownload := metadata.AccountID == "" || metadata.PlanType == ""
	if !isCPAIDTokenMetadataUsable(file["id_token"]) {
		needsDownload = true
	}
	if !needsDownload {
		return metadata, false, nil
	}

	name := mapString(file, "name")
	if name == "" {
		if metadata.AccountID != "" {
			return metadata, false, nil
		}
		return metadata, false, errors.New("CPA auth file name is missing")
	}
	raw, err := doCPAManagementRequest(
		ctx,
		http.MethodGet,
		"/v0/management/auth-files/download?name="+url.QueryEscape(name),
		nil,
	)
	if err != nil {
		if metadata.AccountID != "" {
			return metadata, false, nil
		}
		return metadata, false, err
	}
	metadata = mergeCPAMetadata(metadata, ExtractCPAAuthMetadata(raw))
	if metadata.AccountID == "" {
		return metadata, false, errors.New("CPA account id is missing")
	}
	patch := BuildCPAAuthMetadataPatch(name, raw, metadata)
	if len(patch) == 0 {
		return metadata, false, nil
	}
	if _, err := doCPAManagementRequest(ctx, http.MethodPatch, "/v0/management/auth-files/fields", patch); err != nil {
		return metadata, false, nil
	}
	return metadata, true, nil
}

func BuildCPAAuthMetadataPatch(name string, raw map[string]any, metadata CPAAuthMetadata) map[string]any {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	patch := map[string]any{"name": name}
	if metadata.AccountID != "" {
		patch["account_id"] = metadata.AccountID
		patch["chatgpt_account_id"] = metadata.AccountID
	}
	if metadata.PlanType != "" {
		patch["plan_type"] = metadata.PlanType
		patch["chatgpt_plan_type"] = metadata.PlanType
	}
	if metadata.Email != "" {
		patch["email"] = metadata.Email
	}
	idToken := firstCPAString(raw, "id_token")
	if !IsCLIProxyCompatibleCPAIDToken(idToken) {
		if accessToken := firstCPAString(raw, "access_token"); IsCLIProxyCompatibleCPAIDToken(accessToken) {
			patch["id_token"] = accessToken
		}
	}
	if len(patch) == 1 {
		return nil
	}
	return patch
}

func IsCLIProxyCompatibleCPAIDToken(token string) bool {
	claims := decodeCPAJWTClaims(token)
	if len(claims) == 0 {
		return false
	}
	if _, ok := claims["aud"].(string); ok {
		return false
	}
	return metadataFromCPAClaims(claims).AccountID != ""
}

func IsCLIProxyCompatibleCPAToken(token string) bool {
	claims := decodeCPAJWTClaims(token)
	if len(claims) == 0 {
		return false
	}
	_, hasStringAudience := claims["aud"].(string)
	return !hasStringAudience
}

func isCPAIDTokenMetadataUsable(value any) bool {
	switch token := value.(type) {
	case string:
		return IsCLIProxyCompatibleCPAIDToken(token)
	case map[string]any:
		return metadataFromCPAClaims(token).AccountID != ""
	default:
		return false
	}
}

func metadataFromCPAJWT(token string) CPAAuthMetadata {
	return metadataFromCPAClaims(decodeCPAJWTClaims(token))
}

func decodeCPAJWTClaims(token string) map[string]any {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return map[string]any{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return map[string]any{}
	}
	claims := map[string]any{}
	if err := common.Unmarshal(payload, &claims); err != nil {
		return map[string]any{}
	}
	return claims
}

func metadataFromCPAClaims(claims map[string]any) CPAAuthMetadata {
	auth := mapObject(claims["https://api.openai.com/auth"])
	profile := mapObject(claims["https://api.openai.com/profile"])
	email := firstCPAString(claims, "email", "preferred_username", "upn", "unique_name", "name")
	if email == "" {
		email = firstCPAString(profile, "email")
	}
	metadata := CPAAuthMetadata{
		AccountID: firstCPAString(claims, "chatgpt_account_id", "account_id"),
		PlanType:  firstCPAString(claims, "chatgpt_plan_type", "plan_type"),
		Email:     email,
	}
	return mergeCPAMetadata(metadata, CPAAuthMetadata{
		AccountID: firstCPAString(auth, "chatgpt_account_id", "account_id"),
		PlanType:  firstCPAString(auth, "chatgpt_plan_type", "plan_type"),
	})
}

func mergeCPAMetadata(metadata CPAAuthMetadata, candidate CPAAuthMetadata) CPAAuthMetadata {
	if metadata.AccountID == "" {
		metadata.AccountID = candidate.AccountID
	}
	if metadata.PlanType == "" {
		metadata.PlanType = candidate.PlanType
	}
	if metadata.Email == "" {
		metadata.Email = candidate.Email
	}
	return metadata
}

func firstCPAString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return ""
}
