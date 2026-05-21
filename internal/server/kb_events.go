package server

import "github.com/ContinuumApp/continuum-plugin-support/internal/store"

// kbPublishEvent is the helper Unit 12 (Phase G) replaces with the
// real host.PublishEvent call. Stubbed here so the admin handlers
// compile before the event-publisher wiring lands.
func kbPublishEvent(_ Deps, _ string, _ store.KBArticle, _ map[string]any) {}
