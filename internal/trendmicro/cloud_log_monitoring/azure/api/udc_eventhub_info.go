package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"

	"github.com/trend-vcs/terraform-provider-vision-one/internal/trendmicro"
)

// EventHubStacksPath is Cloud Log Monitoring's Azure Event Hub stack registration endpoint,
// reached through Vision One's Service Platform gateway at the provider's configured
// regional_fqdn. The doubled "scm" segment is real: "/external/v2/direct/scm" is the gateway's
// mount point for the whole CLM service, and "/external/scm/api/eventhub/stacks" is CLM's own
// path underneath it.
const EventHubStacksPath = "/external/v2/direct/scm/external/scm/api/eventhub/stacks"

// CloudProviderAzure is the only provider this package registers.
const CloudProviderAzure = "azure"

// UdcEventHubInfo is the wire representation of a stack registration request. The backend stores one
// row per (tenant, subscription, region) and carries the whole namespace topology in Details, so
// a subscription has exactly one of these however many vendors it collects. Registering a second
// vendor for the same key overwrites Details wholesale rather than merging into it, so a caller
// onboarding a new vendor must resubmit every namespace already registered, not just the new one.
//
// CloudRegion and CloudParentStackRegion are both the CAM region and are sent identically; they
// are separate fields because the backend reserves the option of a CLM region that no longer
// inherits from CAM.
//
// The backend never echoes this back — its response carries only a human-readable message — so
// UdcEventHubInfo is write-only from the provider's perspective.
type UdcEventHubInfo struct {
	CloudProvider          string  `json:"cloudProvider"`
	CloudTenantID          string  `json:"cloudTenantId"`
	CloudAccountID         string  `json:"cloudAccountId"`
	CloudRegion            string  `json:"cloudRegion"`
	CloudParentStackRegion string  `json:"cloudParentStackRegion"`
	Details                Details `json:"details"`
}

// Details is the provisioned Event Hub topology.
type Details struct {
	ResourceGroup      string      `json:"resourceGroup"`
	EventHubNamespaces []Namespace `json:"eventHubNamespaces"`
}

// Namespace is one Event Hub namespace and the hubs inside it.
type Namespace struct {
	Name      string     `json:"name"`
	EventHubs []EventHub `json:"eventHubs"`
}

// EventHub is one hub and the consumer groups Vision One may read from it. The vendor is not a
// field: the backend derives it from the hub name, which is why the naming scheme is part of the
// contract.
type EventHub struct {
	Name           string   `json:"name"`
	ConsumerGroups []string `json:"consumerGroups"`
}

// ClmClient wraps the shared Vision One client for Cloud Log Monitoring calls. Every call
// authenticates with a caller-supplied device token instead of the client's provider-level API
// key: a device token is scoped to one customer's one subscription, so it cannot be the shared,
// provider-wide credential.
type ClmClient struct {
	Client *trendmicro.Client
}

// do sends the request with the given device token and returns an error unless the backend
// answers 200. Every route here responds with a small JSON message on success; the caller
// doesn't need the body back, only whether the call succeeded.
func (c *ClmClient) do(req *http.Request, deviceToken string) error {
	res, err := c.Client.DoRequestRawWithToken(req, deviceToken)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusOK {
		return nil
	}

	traceID := res.Header.Get("x-trace-id")
	var out bytes.Buffer
	if jsonErr := json.Indent(&out, body, "", "  "); jsonErr != nil {
		return fmt.Errorf("unexpected status %d: %s \nTrace id: %s", res.StatusCode, body, traceID)
	}

	return fmt.Errorf("unexpected status %d: \n%s \nTrace id: %s", res.StatusCode, out.String(), traceID)
}

// UpsertUdcEventHubInfo registers the Event Hub stack, or overwrites the existing record's topology
// when one already carries the same tenant, subscription and region. There is no separate update
// endpoint — this is also what backs Update.
func (c *ClmClient) UpsertUdcEventHubInfo(deviceToken string, payload *UdcEventHubInfo) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.Client.HostURL+EventHubStacksPath, bytes.NewBuffer(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req, deviceToken)
}

// DeleteUdcEventHubInfo removes every record registered for the subscription. The backend treats a
// subscription with nothing registered as success, so teardown is idempotent.
func (c *ClmClient) DeleteUdcEventHubInfo(deviceToken, subscriptionID string) error {
	url := c.Client.HostURL + EventHubStacksPath + "/" + neturl.PathEscape(subscriptionID)

	req, err := http.NewRequest(http.MethodDelete, url, http.NoBody)
	if err != nil {
		return err
	}

	return c.do(req, deviceToken)
}
