package grpcclient

import (
	"context"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// RequestClient implements usecase.RequestLookupClient against request-service's internal
// LookupRequestBySource RPC. The connection must carry internalcaller.ClientInterceptor: the RPC is
// refused without the shared token. Only a found flag comes back, never the Request's content.
type RequestClient struct {
	client requestv1.RequestServiceClient
}

func NewRequestClient(client requestv1.RequestServiceClient) *RequestClient {
	return &RequestClient{client: client}
}

func (c *RequestClient) Lookup(ctx context.Context, tenantID, provider, site, ref string) (bool, error) {
	ctx = withTenantMetadata(ctx, tenantID)
	resp, err := c.client.LookupRequestBySource(ctx, &requestv1.LookupRequestBySourceRequest{Provider: provider, Site: site, Ref: ref})
	if err != nil {
		return false, err
	}
	return resp.GetFound(), nil
}
