//go:build !ignore_uncovered
// +build !ignore_uncovered

// Copyright 2020 Microsoft. All rights reserved.
// MIT License

package fakes

import (
	"context"

	"github.com/Azure/azure-container-networking/cns/nmagent"
	nma "github.com/Azure/azure-container-networking/nmagent"
)

// NMAgentClientFake can be used to query to VM Host info.
type NMAgentClientFake struct {
	GetNCVersionListFunc func(context.Context) (nma.NCVersionList, error)
	SupportedAPIsF       func(context.Context) ([]string, error)
	GetHomeAzF           func(context.Context) (nma.AzResponse, error)
}

func (c *NMAgentClientFake) GetNCVersionList(ctx context.Context) (nma.NCVersionList, error) {
	return c.GetNCVersionListFunc(ctx)
}

func (n *NMAgentClientFake) SupportedAPIs(ctx context.Context) ([]string, error) {
	return n.SupportedAPIsF(ctx)
}

func (n *NMAgentClientFake) GetHomeAz(ctx context.Context) (nma.AzResponse, error) {
	return n.GetHomeAzF(ctx)
}

type LegacyNMAgentClientFake struct {
	GetNCVersionListFunc func(context.Context) (*nmagent.NetworkContainerListResponse, error)
	SupportedAPIsF       func(context.Context) ([]string, error)
	GetHomeAzF           func(context.Context) (nma.AzResponse, error)
}

func (c *LegacyNMAgentClientFake) GetNCVersionList(ctx context.Context) (*nmagent.NetworkContainerListResponse, error) {
	return c.GetNCVersionListFunc(ctx)
}

func (n *LegacyNMAgentClientFake) SupportedAPIs(ctx context.Context) ([]string, error) {
	return n.SupportedAPIsF(ctx)
}

func (n *LegacyNMAgentClientFake) GetHomeAz(ctx context.Context) (nma.AzResponse, error) {
	return n.GetHomeAzF(ctx)
}
