package usecase

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ExportFleetDefinitionYaml serializes a saved FleetDefinition back into
// orca-fleet.yaml's FleetConfigSchema shape
// (frontend/src/shared/fleet-config-parser.ts) — CR-FLEET-003's AC is a
// round-trip: export then re-parse via FleetConfigSchema should produce an
// equivalent FleetSpec.
//
// KNOWN GAP (not this task's to fix): at the time this was written,
// FleetServerSchema (fleet-config-parser.ts) has NO `vaultSshRole` field —
// TASK-BE-FLEET-005 (adding it) is out of scope for the agent that wrote
// this file (frontend, outside its allowed directories) and was SKIPPED.
// This usecase still emits `vaultSshRole` per this task's spec (the field
// name/shape CR-FLEET-003 calls for), but until TASK-BE-FLEET-005 lands,
// re-importing the exported YAML through today's real FleetServerSchema
// silently drops that key (Zod strips unrecognized object keys by
// default) — the round-trip AC does NOT fully hold end-to-end yet. See
// this task's "Kết quả thực tế" for the full note.
type ExportFleetDefinitionYaml struct {
	repo FleetDefinitionRepository
}

func NewExportFleetDefinitionYaml(repo FleetDefinitionRepository) *ExportFleetDefinitionYaml {
	return &ExportFleetDefinitionYaml{repo: repo}
}

func (uc *ExportFleetDefinitionYaml) Execute(ctx context.Context, id string) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	def, err := uc.repo.Get(ctx, tenantID, id)
	if err != nil {
		if err == domain.ErrFleetDefinitionNotFound {
			return "", apperrors.New(apperrors.KindNotFound, "INFRA_FLEET_DEFINITION_NOT_FOUND", "fleet definition not found", err)
		}
		return "", apperrors.New(apperrors.KindInternal, "INFRA_GET_FLEET_DEFINITION_FAILED", "failed to get fleet definition", err)
	}
	out, err := serializeFleetYaml(def)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "INFRA_EXPORT_FLEET_DEFINITION_YAML_FAILED", "failed to serialize fleet definition yaml", err)
	}
	return out, nil
}

type yamlFleetServer struct {
	ID           string `yaml:"id"`
	Label        string `yaml:"label"`
	Host         string `yaml:"host"`
	Username     string `yaml:"username,omitempty"`
	VaultSSHRole string `yaml:"vaultSshRole,omitempty"`
}

type yamlFleetProvision struct {
	IaC        string `yaml:"iac"`
	WorkingDir string `yaml:"workingDir"`
	VarsFile   string `yaml:"varsFile,omitempty"`
}

type yamlFleetConfig struct {
	Version   string              `yaml:"version"`
	Servers   []yamlFleetServer   `yaml:"servers"`
	Provision *yamlFleetProvision `yaml:"provision,omitempty"`
}

// serializeFleetYaml maps domain.FleetDefinition -> orca-fleet.yaml's
// FleetConfigSchema (frontend/src/shared/fleet-config-parser.ts).
// FleetServerSchema requires `id`/`label` (not present on
// domain.FleetSpecServer) — synthesized here as host-derived values so
// the exported YAML still validates against FleetConfigSchema's required
// fields on re-import. This is a lossy synthesis (id/label are invented,
// not round-tripped from anything the user originally typed) — acceptable
// per CR-FLEET-003's AC ("round-trip: parse lại ra FleetSpec tương
// đương", not "byte-identical id/label").
func serializeFleetYaml(def domain.FleetDefinition) (string, error) {
	cfg := yamlFleetConfig{Version: "1"}
	for i, s := range def.Servers {
		cfg.Servers = append(cfg.Servers, yamlFleetServer{
			ID:           fmt.Sprintf("%s-%d", def.ID, i), // synthesized — see doc comment above
			Label:        s.Host,                          // Host used as the default label — domain has no separate label field
			Host:         s.Host,
			Username:     s.UserName,
			VaultSSHRole: s.VaultSSHRole,
		})
	}
	if def.Provision != nil {
		cfg.Provision = &yamlFleetProvision{
			IaC: def.Provision.IaC, WorkingDir: def.Provision.WorkingDir, VarsFile: def.Provision.VarsFile,
		}
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
