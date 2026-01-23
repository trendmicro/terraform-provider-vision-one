package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"

	cloud_risk_management_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"
)

const (
	profilesPath = "/beta/cloudPosture/profiles"
)

func extractIDFromLocation(respHeader *http.Header) (string, error) {
	location := respHeader.Get("Location")

	parsedURL, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %v", err)
	}

	// Use the path.Base function to get the last element of the path
	return path.Base(parsedURL.Path), nil
}

// CreateProfile creates a new profile
func (c *CrmClient) CreateProfile(req cloud_risk_management_dto.CreateProfileRequest) (*cloud_risk_management_dto.Profile, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create profile request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s%s", c.HostURL, profilesPath), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	response, err := c.DoRequestWithFullResponse(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}
	defer response.Body.Close()

	// try to extract the profile ID from response's Location header
	id, err := extractIDFromLocation(&response.Header)
	if err != nil {
		return nil, err
	}

	if id == "" {
		return nil, fmt.Errorf("failed to extract profile ID from response header")
	}

	// API returns 201 with empty body, need to list profiles to get the created one
	// For now, return a profile with the name from request
	profile := &cloud_risk_management_dto.Profile{
		Name:        req.Name,
		Description: req.Description,
		ScanRules:   req.ScanRules,
	}
	profile.ID = id

	return profile, nil
}

// GetProfile retrieves a profile by ID
func (c *CrmClient) GetProfile(profileID string) (*cloud_risk_management_dto.Profile, error) {
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s%s/%s", c.HostURL, profilesPath, profileID), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	body, err := c.DoRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	var profile cloud_risk_management_dto.Profile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile response: %w", err)
	}

	profile.ID = profileID

	return &profile, nil
}

// UpdateProfile updates an existing profile
func (c *CrmClient) UpdateProfile(profileID string, req cloud_risk_management_dto.UpdateProfileRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal update profile request: %w", err)
	}

	httpReq, err := http.NewRequest("PATCH", fmt.Sprintf("%s%s/%s", c.HostURL, profilesPath, profileID), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("TMV1-Patch-Array-Mode", "update")
	httpReq.Header.Set("Content-Type", "application/json")

	_, err = c.DoRequest(httpReq)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	return nil
}

// DeleteProfile deletes a profile by ID
func (c *CrmClient) DeleteProfile(profileID string) error {
	httpReq, err := http.NewRequest("DELETE", fmt.Sprintf("%s%s/%s", c.HostURL, profilesPath, profileID), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	_, err = c.DoRequest(httpReq)
	if err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	return nil
}
