// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package huawei

import (
	"strings"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	billing_api "yunion.io/x/cloudmux/pkg/apis/billing"
	api "yunion.io/x/cloudmux/pkg/apis/compute"
	"yunion.io/x/cloudmux/pkg/cloudprovider"
	"yunion.io/x/cloudmux/pkg/multicloud"
)

type SNatGateway struct {
	multicloud.SNatGatewayBase
	HuaweiTags
	region *SRegion

	ID                string
	Name              string
	Description       string
	Spec              string
	Status            string
	InternalNetworkId string
	CreatedTime       string `json:"created_at"`
}

func (gateway *SNatGateway) GetId() string {
	return gateway.ID
}

func (gateway *SNatGateway) GetName() string {
	return gateway.Name
}

func (gateway *SNatGateway) GetGlobalId() string {
	return gateway.GetId()
}

func (gateway *SNatGateway) GetStatus() string {
	return NatResouceStatusTransfer(gateway.Status)
}

func (self *SNatGateway) Delete() error {
	return self.region.DeleteNatGateway(self.ID)
}

func (self *SNatGateway) Refresh() error {
	nat, err := self.region.GetNatGateway(self.ID)
	if err != nil {
		return errors.Wrapf(err, "GetNatGateway(%s)", self.ID)
	}
	return jsonutils.Update(self, nat)
}

func (self *SNatGateway) GetINetworkId() string {
	return self.InternalNetworkId
}

func (gateway *SNatGateway) GetNatSpec() string {
	switch gateway.Spec {
	case "1":
		return api.NAT_SPEC_SMALL
	case "2":
		return api.NAT_SPEC_MIDDLE
	case "3":
		return api.NAT_SPEC_LARGE
	case "4":
		return api.NAT_SPEC_XLARGE
	}
	return gateway.Spec
}

func (gateway *SNatGateway) GetDescription() string {
	return gateway.Description
}

func (gateway *SNatGateway) GetBillingType() string {
	// Up to 2019.07.17, only support post pay
	return billing_api.BILLING_TYPE_POSTPAID
}

func (gateway *SNatGateway) GetCreatedAt() time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05.000000", gateway.CreatedTime)
	return t
}

func (gateway *SNatGateway) GetExpiredAt() time.Time {
	// no support for expired time
	return time.Time{}
}

func (gateway *SNatGateway) GetIEips() ([]cloudprovider.ICloudEIP, error) {
	IEips, err := gateway.region.GetIEips()
	if err != nil {
		return nil, errors.Wrapf(err, `get all Eips of region %q error`, gateway.region.GetId())
	}
	dNatTables, err := gateway.GetINatDTable()
	if err != nil {
		return nil, errors.Wrapf(err, `get all DNatTable of gateway %q error`, gateway.GetId())
	}
	sNatTables, err := gateway.GetINatSTable()
	if err != nil {
		return nil, errors.Wrapf(err, `get all SNatTable of gateway %q error`, gateway.GetId())
	}

	// Get natIPSet of nat rules
	natIPSet := make(map[string]struct{})
	for _, snat := range sNatTables {
		natIPSet[snat.GetIP()] = struct{}{}
	}
	for _, dnat := range dNatTables {
		natIPSet[dnat.GetExternalIp()] = struct{}{}
	}

	// Add Eip whose GetIpAddr() in natIPSet to ret
	ret := make([]cloudprovider.ICloudEIP, 0, 2)
	for i := range IEips {
		if _, ok := natIPSet[IEips[i].GetIpAddr()]; ok {
			ret = append(ret, IEips[i])
		}
	}
	return ret, nil
}

func (gateway *SNatGateway) GetINatDTable() ([]cloudprovider.ICloudNatDEntry, error) {
	dNatTable, err := gateway.getNatDTable()
	if err != nil {
		return nil, errors.Wrapf(err, `get dnat table of nat gateway %q`, gateway.GetId())
	}
	ret := make([]cloudprovider.ICloudNatDEntry, len(dNatTable))
	for i := range dNatTable {
		ret[i] = &dNatTable[i]
	}
	return ret, nil
}

func (gateway *SNatGateway) GetINatSTable() ([]cloudprovider.ICloudNatSEntry, error) {
	sNatTable, err := gateway.getNatSTable()
	if err != nil {
		return nil, errors.Wrapf(err, `get dnat table of nat gateway %q`, gateway.GetId())
	}
	ret := make([]cloudprovider.ICloudNatSEntry, len(sNatTable))
	for i := range sNatTable {
		ret[i] = &sNatTable[i]
	}
	return ret, nil
}

func (gateway *SNatGateway) CreateINatDEntry(rule cloudprovider.SNatDRule) (cloudprovider.ICloudNatDEntry, error) {
	dnat, err := gateway.region.CreateNatDEntry(rule, gateway.GetId())
	if err != nil {
		return nil, err
	}
	dnat.gateway = gateway
	return &dnat, nil
}

func (gateway *SNatGateway) CreateINatSEntry(rule cloudprovider.SNatSRule) (cloudprovider.ICloudNatSEntry, error) {
	snat, err := gateway.region.CreateNatSEntry(rule, gateway.GetId())
	if err != nil {
		return nil, err
	}
	snat.gateway = gateway
	return &snat, nil
}

func (gateway *SNatGateway) GetINatDEntryByID(id string) (cloudprovider.ICloudNatDEntry, error) {
	dnat, err := gateway.region.GetNatDEntryByID(id)
	if err != nil {
		return nil, err
	}
	dnat.gateway = gateway
	return &dnat, nil
}

func (gateway *SNatGateway) GetINatSEntryByID(id string) (cloudprovider.ICloudNatSEntry, error) {
	snat, err := gateway.region.GetNatSEntryByID(id)
	if err != nil {
		return nil, err
	}
	snat.gateway = gateway
	return &snat, nil
}

func (region *SRegion) GetNatGateways(vpcID, natGatewayID string) ([]SNatGateway, error) {
	queues := make(map[string]string)
	if len(natGatewayID) != 0 {
		queues["id"] = natGatewayID
	}
	if len(vpcID) != 0 {
		queues["router_id"] = vpcID
	}
	natGateways := make([]SNatGateway, 0, 2)
	err := doListAllWithMarker(region.ecsClient.NatGateways.List, queues, &natGateways)
	if err != nil {
		return nil, errors.Wrapf(err, "get nat gateways error by natgatewayid")
	}
	for i := range natGateways {
		natGateways[i].region = region
	}
	return natGateways, nil
}

func (region *SRegion) CreateNatDEntry(rule cloudprovider.SNatDRule, gatewayID string) (SNatDEntry, error) {
	params := make(map[string]interface{})
	params["nat_gateway_id"] = gatewayID
	params["private_ip"] = rule.InternalIP
	params["internal_service_port"] = rule.InternalPort
	params["floating_ip_id"] = rule.ExternalIPID
	params["external_service_port"] = rule.ExternalPort
	params["protocol"] = rule.Protocol

	packParams := map[string]map[string]interface{}{
		"dnat_rule": params,
	}

	ret := SNatDEntry{}
	err := DoCreate(region.ecsClient.DNatRules.Create, jsonutils.Marshal(packParams), &ret)
	if err != nil {
		return SNatDEntry{}, errors.Wrapf(err, `create dnat rule of nat gateway %q failed`, gatewayID)
	}
	return ret, nil
}

func (region *SRegion) CreateNatSEntry(rule cloudprovider.SNatSRule, gatewayID string) (SNatSEntry, error) {
	params := make(map[string]interface{})
	params["nat_gateway_id"] = gatewayID
	if len(rule.NetworkID) != 0 {
		params["network_id"] = rule.NetworkID
	}
	if len(rule.SourceCIDR) != 0 {
		params["cidr"] = rule.SourceCIDR
	}
	params["floating_ip_id"] = rule.ExternalIPID

	packParams := map[string]map[string]interface{}{
		"snat_rule": params,
	}

	ret := SNatSEntry{}
	err := DoCreate(region.ecsClient.SNatRules.Create, jsonutils.Marshal(packParams), &ret)
	if err != nil {
		return SNatSEntry{}, errors.Wrapf(err, `create snat rule of nat gateway %q failed`, gatewayID)
	}
	return ret, nil
}

func (region *SRegion) GetNatDEntryByID(id string) (SNatDEntry, error) {
	dnat := SNatDEntry{}
	err := DoGet(region.ecsClient.DNatRules.Get, id, map[string]string{}, &dnat)

	if err != nil {
		return SNatDEntry{}, err
	}
	return dnat, nil
}

func (region *SRegion) GetNatSEntryByID(id string) (SNatSEntry, error) {
	snat := SNatSEntry{}
	err := DoGet(region.ecsClient.SNatRules.Get, id, map[string]string{}, &snat)
	if err != nil {
		return SNatSEntry{}, cloudprovider.ErrNotFound
	}
	return snat, nil
}

func NatResouceStatusTransfer(status string) string {
	// In Huawei Cloud, there are isx resource status of Nat, "ACTIVE", "PENDING_CREATE",
	// "PENDING_UPDATE", "PENDING_DELETE", "EIP_FREEZED", "INACTIVE".
	switch status {
	case "ACTIVE":
		return api.NAT_STAUTS_AVAILABLE
	case "PENDING_CREATE":
		return api.NAT_STATUS_ALLOCATE
	case "PENDING_UPDATE", "PENDING_DELETE":
		return api.NAT_STATUS_DEPLOYING
	default:
		return api.NAT_STATUS_UNKNOWN
	}
}

func (self *SRegion) GetNatGateway(id string) (*SNatGateway, error) {
	resp, err := self.ecsClient.NatGateways.Get(id, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "NatGateways.Get(%s)", id)
	}
	nat := &SNatGateway{region: self}
	err = resp.Unmarshal(nat)
	return nat, errors.Wrap(err, "resp.Unmarshal")
}

func (self *SVpc) CreateINatGateway(opts *cloudprovider.NatGatewayCreateOptions) (cloudprovider.ICloudNatGateway, error) {
	nat, err := self.region.CreateNatGateway(opts)
	if err != nil {
		return nil, errors.Wrapf(err, "CreateNatGateway")
	}
	return nat, nil
}

func (self *SRegion) CreateNatGateway(opts *cloudprovider.NatGatewayCreateOptions) (*SNatGateway, error) {
	spec := ""
	switch strings.ToLower(opts.NatSpec) {
	case api.NAT_SPEC_SMALL:
		spec = "1"
	case api.NAT_SPEC_MIDDLE:
		spec = "2"
	case api.NAT_SPEC_LARGE:
		spec = "3"
	case api.NAT_SPEC_XLARGE:
		spec = "4"
	}
	params := jsonutils.Marshal(map[string]map[string]interface{}{
		"nat_gateway": map[string]interface{}{
			"name":                opts.Name,
			"description":         opts.Desc,
			"router_id":           opts.VpcId,
			"internal_network_id": opts.NetworkId,
			"spec":                spec,
		},
	})
	resp, err := self.ecsClient.NatGateways.Create(params)
	if err != nil {
		return nil, errors.Wrap(err, "AsyncCreate")
	}
	nat := &SNatGateway{region: self}
	err = resp.Unmarshal(nat)
	if err != nil {
		return nil, errors.Wrapf(err, "resp.Unmarshal")
	}
	return nat, nil
}

func (self *SRegion) DeleteNatGateway(id string) error {
	_, err := self.ecsClient.NatGateways.Delete(id, nil)
	return errors.Wrapf(err, "NatGateways.Delete(%s)", id)
}
