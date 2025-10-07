package api

import (
	"terraform-provider-vision-one/internal/trendmicro"
)

type CamClient struct {
	Client *trendmicro.Client
}
