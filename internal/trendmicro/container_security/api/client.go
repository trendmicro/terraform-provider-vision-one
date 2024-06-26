package api

import "terraform-provider-visionone/internal/trendmicro"

type CsClient struct {
	Client *trendmicro.Client
}
