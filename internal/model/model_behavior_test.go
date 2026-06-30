package model

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAgentRoleCompatibilityAndIdentity(t *testing.T) {
	agentRoles := []AgentRole{
		AgentRoleAgent,
		AgentRoleRequirement,
		AgentRoleArchitect,
		AgentRoleDeveloper,
		AgentRoleReviewer,
		AgentRoleTestEngineer,
		AgentRoleDevOps,
		AgentRoleFullstackEngineer,
		AgentRoleCustom,
	}
	for _, role := range agentRoles {
		if !role.IsAgentRole() || !role.IsValid() || role.NormalizeRole() != AgentRoleAgent {
			t.Fatalf("agent role compatibility failed for %q", role)
		}
	}
	if !AgentRoleHuman.IsHumanRole() || !AgentRoleHuman.IsValid() || AgentRoleHuman.NormalizeRole() != AgentRoleHuman {
		t.Fatalf("human role compatibility failed")
	}
	if AgentRole("unknown").IsValid() || AgentRole("unknown").IsAgentRole() || AgentRole("unknown").NormalizeRole() != AgentRoleAgent {
		t.Fatalf("unknown role handling changed")
	}

	id := uuid.New()
	reviewerA := &AgentRoleConfig{ID: id, Role: AgentRoleReviewer}
	reviewerB := &AgentRoleConfig{ID: uuid.New(), Role: AgentRoleReviewer}
	developer := &AgentRoleConfig{ID: id, Role: AgentRoleDeveloper}
	if !reviewerA.IsSameRole(reviewerB) || reviewerA.IsSameRole(developer) {
		t.Fatalf("IsSameRole mismatch")
	}
	if !reviewerA.IsSameAgent(developer) || reviewerA.IsSameAgent(reviewerB) {
		t.Fatalf("IsSameAgent mismatch")
	}
	if reviewerA.IsSameRole(nil) || (*AgentRoleConfig)(nil).IsSameRole(reviewerA) {
		t.Fatalf("IsSameRole should handle nil receivers")
	}
	if reviewerA.IsSameAgent(nil) || (*AgentRoleConfig)(nil).IsSameAgent(reviewerA) {
		t.Fatalf("IsSameAgent should handle nil receivers")
	}
}

func TestCreateProjectRequestValidate(t *testing.T) {
	valid := &CreateProjectRequest{Name: "Colink", LocalPath: "/tmp/colink", Mode: ProjectModeNew}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request returned error: %v", err)
	}

	cases := []struct {
		name  string
		req   *CreateProjectRequest
		field string
	}{
		{name: "missing name", req: &CreateProjectRequest{LocalPath: "/tmp/colink"}, field: "name"},
		{name: "missing local path", req: &CreateProjectRequest{Name: "Colink"}, field: "localPath"},
		{name: "enhance missing repo", req: &CreateProjectRequest{Name: "Colink", LocalPath: "/tmp/colink", Mode: ProjectModeEnhance}, field: "existingRepoUrl"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if err == nil {
				t.Fatalf("Validate should fail")
			}
			validationErr, ok := err.(*ValidationError)
			if !ok || validationErr.Field != tt.field || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("Validate error = %#v", err)
			}
		})
	}
}

func TestSessionRecordLifecycleHelpers(t *testing.T) {
	record := &SessionRecord{}
	if err := record.BeforeCreate(); err != nil {
		t.Fatalf("BeforeCreate returned error: %v", err)
	}
	if record.ID == uuid.Nil || record.CreatedAt == 0 || record.UpdatedAt == 0 || record.LastActiveAt == 0 {
		t.Fatalf("BeforeCreate did not initialize fields: %#v", record)
	}
	createdAt := record.CreatedAt
	record.LastActiveAt = 123
	if err := record.BeforeCreate(); err != nil {
		t.Fatalf("BeforeCreate second call returned error: %v", err)
	}
	if record.LastActiveAt == 0 {
		t.Fatalf("BeforeCreate should preserve existing LastActiveAt")
	}

	time.Sleep(time.Second)
	if err := record.BeforeUpdate(); err != nil {
		t.Fatalf("BeforeUpdate returned error: %v", err)
	}
	if record.UpdatedAt <= createdAt {
		t.Fatalf("BeforeUpdate did not advance UpdatedAt: %#v", record)
	}

	record.ResumeExpiry = 0
	if record.IsExpired(24) {
		t.Fatalf("zero ResumeExpiry should not be expired")
	}
	record.ResumeExpiry = time.Now().Add(-time.Hour).Unix()
	if !record.IsExpired(24) {
		t.Fatalf("past ResumeExpiry should be expired")
	}
	record.SetResumeExpiry(1)
	if record.ResumeExpiry <= time.Now().Unix() || record.IsExpired(1) {
		t.Fatalf("SetResumeExpiry did not set future expiry: %d", record.ResumeExpiry)
	}
}
