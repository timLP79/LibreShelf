// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

// SiteFooter is the per-deployment attribution state injected into every
// rendered page via renderTemplate. Computed once at startup so templates
// do not re-check env or DB on every render.
type SiteFooter struct {
	GoogleBooksConfigured bool
	OfflineLocked         bool
}

// siteFooter is the shared attribution state. Read in renderTemplate.
// Written only at startup via initSiteFooter.
var siteFooter SiteFooter

// initSiteFooter captures the deployment's attribution state. Called
// once from main() after package init() functions have run.
func initSiteFooter() {
	siteFooter = SiteFooter{
		GoogleBooksConfigured: IsGoogleBooksConfigured(),
		OfflineLocked:         IsOfflineEnvLocked(),
	}
}
