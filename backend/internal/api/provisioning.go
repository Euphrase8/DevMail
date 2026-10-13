package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Euphrase8/DevMail/backend/internal/dns"
	"github.com/Euphrase8/DevMail/backend/internal/domains"
	"github.com/Euphrase8/DevMail/backend/internal/engine"
	"github.com/Euphrase8/DevMail/backend/internal/jobs"
	"github.com/Euphrase8/DevMail/backend/internal/mailboxes"
)

// idFrom decodes a job payload of the form {"<key>":"<value>"} and returns the
// value. Payloads carry only database ids and non-secret identifiers; passwords
// are hashed at rest and never placed in the queue.
func idFrom(payload []byte, key string) (string, error) {
	var m map[string]string
	if err := json.Unmarshal(payload, &m); err != nil {
		return "", fmt.Errorf("invalid job payload: %w", err)
	}
	v := m[key]
	if v == "" {
		return "", fmt.Errorf("job payload missing %q", key)
	}
	return v, nil
}

// registerProvisioningConsumers binds real consumers to the shared jobs service
// so mutations actually provision against the configured mail engine.
func registerProvisioningConsumers(
	jobsSvc *jobs.Service,
	eng engine.Engine,
	mb *mailboxes.Service,
	dom *domains.Service,
	dnsSvc *dns.Service,
) {
	jobsSvc.Register("dns.apply", func(ctx context.Context, payload []byte) (any, error) {
		domainID, err := idFrom(payload, "domainId")
		if err != nil {
			return nil, err
		}
		providerID, err := idFrom(payload, "providerId")
		if err != nil {
			return nil, err
		}
		if _, err := dnsSvc.Plan(ctx, domainID); err != nil {
			return nil, err
		}
		records, results, err := dnsSvc.Apply(ctx, domainID, providerID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"records": len(records), "results": len(results)}, nil
	})

	// Engine consumers only make sense when a live engine is configured; without
	// one the records stay "unmanaged" and nothing is queued.
	if !eng.Configured() {
		return
	}

	jobsSvc.Register("domain.provision", func(ctx context.Context, payload []byte) (any, error) {
		id, err := idFrom(payload, "domainId")
		if err != nil {
			return nil, err
		}
		name, err := dom.Name(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := eng.EnsureDomain(ctx, name); err != nil {
			_ = dom.MarkProvisioned(ctx, id, err.Error())
			return nil, err
		}
		if err := dom.MarkProvisioned(ctx, id, ""); err != nil {
			return nil, err
		}
		return map[string]any{"domain": name}, nil
	})

	jobsSvc.Register("domain.remove", func(ctx context.Context, payload []byte) (any, error) {
		id, err := idFrom(payload, "domainId")
		if err != nil {
			return nil, err
		}
		name, err := dom.Name(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := eng.RemoveDomain(ctx, name); err != nil {
			return nil, err
		}
		return map[string]any{"domain": name, "removed": true}, nil
	})

	jobsSvc.Register("mailbox.create", func(ctx context.Context, payload []byte) (any, error) {
		id, err := idFrom(payload, "mailboxId")
		if err != nil {
			return nil, err
		}
		local, domain, hash, quota, err := mb.ProvisionTarget(ctx, id)
		if err != nil {
			return nil, err
		}
		if mb.Deleted(ctx, id) {
			return map[string]any{"removed": true}, eng.DeleteMailbox(ctx, domain, local)
		}
		if err := eng.EnsureDomain(ctx, domain); err != nil {
			return nil, err
		}
		if err := eng.CreateMailbox(ctx, domain, local, hash, quota); err != nil {
			_ = mb.MarkProvisioned(ctx, id, "error", err.Error())
			return nil, err
		}
		if err := mb.MarkProvisioned(ctx, id, "managed", ""); err != nil {
			return nil, err
		}
		return map[string]any{"address": local + "@" + domain}, nil
	})

	jobsSvc.Register("mailbox.password", func(ctx context.Context, payload []byte) (any, error) {
		id, err := idFrom(payload, "mailboxId")
		if err != nil {
			return nil, err
		}
		local, domain, hash, _, err := mb.ProvisionTarget(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := eng.SetPassword(ctx, domain, local, hash); err != nil {
			_ = mb.MarkProvisioned(ctx, id, "error", err.Error())
			return nil, err
		}
		if err := mb.MarkProvisioned(ctx, id, "managed", ""); err != nil {
			return nil, err
		}
		return map[string]any{"address": local + "@" + domain, "passwordUpdated": true}, nil
	})

	jobsSvc.Register("mailbox.delete", func(ctx context.Context, payload []byte) (any, error) {
		id, err := idFrom(payload, "mailboxId")
		if err != nil {
			return nil, err
		}
		local, domain, _, _, err := mb.ProvisionTarget(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := eng.DeleteMailbox(ctx, domain, local); err != nil {
			return nil, err
		}
		if err := mb.MarkProvisioned(ctx, id, "unmanaged", ""); err != nil {
			return nil, err
		}
		return map[string]any{"address": local + "@" + domain, "removed": true}, nil
	})

	jobsSvc.Register("alias.provision", func(ctx context.Context, payload []byte) (any, error) {
		id, err := idFrom(payload, "aliasId")
		if err != nil {
			return nil, err
		}
		if mb.AliasDeleted(ctx, id) {
			return map[string]any{"removed": true}, nil
		}
		source, destination, err := mb.AliasTarget(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := eng.CreateAlias(ctx, source, destination); err != nil {
			return nil, err
		}
		if err := mb.MarkAliasProvisioned(ctx, id); err != nil {
			return nil, err
		}
		return map[string]any{"source": source, "destination": destination}, nil
	})

	jobsSvc.Register("alias.delete", func(ctx context.Context, payload []byte) (any, error) {
		source, err := idFrom(payload, "source")
		if err != nil {
			return nil, err
		}
		if err := eng.DeleteAlias(ctx, source); err != nil {
			return nil, err
		}
		return map[string]any{"source": source, "removed": true}, nil
	})
}
