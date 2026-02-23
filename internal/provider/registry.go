// Package provider implements data providers for fetching media information.
package provider

import (
	"github.com/mydehq/autotitle/internal/types"
)

// providers is the global registry of available providers
var providers []types.Provider

// fillerSources is the global registry of available filler sources
var fillerSources []types.FillerSource

// RegisterProvider adds a provider to the registry
func RegisterProvider(p types.Provider) {
	providers = append(providers, p)
}

// RegisterFillerSource adds a filler source to the registry
func RegisterFillerSource(s types.FillerSource) {
	fillerSources = append(fillerSources, s)
}

// GetProviderForURL finds the provider that can handle the given URL
func GetProviderForURL(url string) (types.Provider, error) {
	for _, p := range providers {
		if p.MatchesURL(url) {
			return p, nil
		}
	}
	return nil, types.ErrProviderNotFound{URL: url}
}

// GetProvider finds a provider by its name
func GetProvider(name string) (types.Provider, error) {
	for _, p := range providers {
		if p.Name() == name {
			return p, nil
		}
	}
	return nil, types.ErrProviderNotFound{URL: name}
}

// GetFillerSourceForURL finds the filler source that can handle the given URL
func GetFillerSourceForURL(url string) (types.FillerSource, error) {
	for _, s := range fillerSources {
		if s.MatchesURL(url) {
			return s, nil
		}
	}
	return nil, types.ErrFillerSourceNotFound{URL: url}
}

// ExtractProviderAndID extracts the provider name and ID from a URL
func ExtractProviderAndID(url string) (provider string, id string, err error) {
	p, err := GetProviderForURL(url)
	if err != nil {
		return "", "", err
	}
	id, err = p.ExtractID(url)
	if err != nil {
		return "", "", err
	}
	return p.Name(), id, nil
}

// ListProviders returns all registered provider names
func ListProviders() []string {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}
	return names
}

// ListFillerSources returns all registered filler source names
func ListFillerSources() []string {
	names := make([]string, len(fillerSources))
	for i, s := range fillerSources {
		names[i] = s.Name()
	}
	return names
}

// FillerSourceInfo holds metadata about a registered filler source
type FillerSourceInfo struct {
	Name      string
	Website   string
	MatchURLs []string
}

// ListFillerSourceDetails returns all registered filler sources with their supported URLs
func ListFillerSourceDetails() []FillerSourceInfo {
	infos := make([]FillerSourceInfo, len(fillerSources))
	for i, s := range fillerSources {
		infos[i] = FillerSourceInfo{Name: s.Name(), Website: s.Website(), MatchURLs: s.SupportedURLs()}
	}
	return infos
}
