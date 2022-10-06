package awsconfig

import (
	"fmt"
	"strings"
)

type Region string

const (
	RegionTokyo         Region = "ap-northeast-1"
	RegionOsaka         Region = "ap-northeast-3"
	RegionOhio          Region = "us-east-1"
	RegionVirginia      Region = "us-east-2"
	RegionCalifornia    Region = "us-west-1"
	RegionOregon        Region = "us-west-2"
	RegionCapeTown      Region = "af-south-1"
	RegionHongKong      Region = "ap-east-1"
	RegionMumbai        Region = "ap-south-1"
	RegionSeoul         Region = "ap-northeast-2"
	RegionSingapore     Region = "ap-southeast-1"
	RegionSydney        Region = "ap-southeast-2"
	RegionCanadaCentral Region = "ca-central-1"
	RegionFrankfurt     Region = "eu-central-1"
	RegionIreland       Region = "eu-west-1"
	RegionLondon        Region = "eu-west-2"
	RegionMilano        Region = "eu-south-1"
	RegionParis         Region = "eu-west-3"
	RegionStockholm     Region = "eu-north-1"
	RegionBahrain       Region = "me-south-1"
	RegionSaoPaulo      Region = "sa-east-1"
)

func (r Region) Value() string {
	if r == "" {
		return string(RegionTokyo)
	}
	return string(r)
}

func ParseRegion(region string) (Region, error) {
	switch v := Region(strings.ToLower(region)); v {
	case RegionTokyo, RegionOsaka, RegionOhio, RegionVirginia, RegionCalifornia, RegionOregon, RegionCapeTown, RegionHongKong,
		RegionMumbai, RegionSeoul, RegionSingapore, RegionSydney, RegionCanadaCentral, RegionFrankfurt, RegionIreland,
		RegionLondon, RegionMilano, RegionParis, RegionStockholm, RegionBahrain, RegionSaoPaulo:
		return v, nil
	default:
		return "", fmt.Errorf("no supported region [%s]", region)
	}
}
