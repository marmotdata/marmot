package team

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/config"
)

type Team struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	CreatedViaSSO bool                   `json:"created_via_sso"`
	SSOProvider   *string                `json:"sso_provider,omitempty"`
	CreatedBy     *string                `json:"created_by,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type TeamMember struct {
	ID          string    `json:"id"`
	TeamID      string    `json:"team_id"`
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"`
	Source      string    `json:"source"`
	SSOProvider *string   `json:"sso_provider,omitempty"`
	JoinedAt    time.Time `json:"joined_at"`
}

type TeamMemberWithUser struct {
	TeamMember
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Email    *string `json:"email,omitempty"`
}

type SSOTeamMapping struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	SSOGroupName string    `json:"sso_group_name"`
	TeamID       string    `json:"team_id"`
	MemberRole   string    `json:"member_role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AssetOwner struct {
	AssetID   string     `json:"asset_id"`
	UserID    *string    `json:"user_id,omitempty"`
	TeamID    *string    `json:"team_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Owner struct {
	Type  string  `json:"type"`
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email,omitempty"`
}

const (
	RoleOwner  = "owner"
	RoleMember = "member"

	SourceManual = "manual"
	SourceSSO    = "sso"

	OwnerTypeUser = "user"
	OwnerTypeTeam = "team"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateTeam(ctx context.Context, name, description, createdBy string) (*Team, error) {
	team := &Team{
		Name:          name,
		Description:   description,
		CreatedViaSSO: false,
		CreatedBy:     &createdBy,
	}

	if err := s.repo.CreateTeam(ctx, team); err != nil {
		return nil, err
	}

	return team, nil
}

func (s *Service) GetTeam(ctx context.Context, id string) (*Team, error) {
	return s.repo.GetTeam(ctx, id)
}

func (s *Service) UpdateTeam(ctx context.Context, id, name, description string) error {
	return s.repo.UpdateTeam(ctx, id, name, description)
}

func (s *Service) UpdateTeamFields(ctx context.Context, id string, name, description *string, metadata map[string]interface{}, tags []string) error {
	return s.repo.UpdateTeamFields(ctx, id, name, description, metadata, tags)
}

func (s *Service) DeleteTeam(ctx context.Context, id string) error {
	return s.repo.DeleteTeam(ctx, id)
}

func (s *Service) ListTeams(ctx context.Context, limit, offset int) ([]*Team, int, error) {
	return s.repo.ListTeams(ctx, limit, offset)
}

func (s *Service) AddMember(ctx context.Context, teamID, userID, role string) error {
	team, err := s.repo.GetTeam(ctx, teamID)
	if err != nil {
		return err
	}

	if team.CreatedViaSSO {
		return ErrCannotEditSSOTeam
	}

	return s.repo.AddMember(ctx, &TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
		Source: SourceManual,
	})
}

func (s *Service) RemoveMember(ctx context.Context, teamID, userID string) error {
	member, err := s.repo.GetMember(ctx, teamID, userID)
	if err != nil {
		return err
	}

	if member.Source == SourceSSO {
		return errors.New("cannot remove SSO-managed member")
	}

	return s.repo.RemoveMember(ctx, teamID, userID)
}

func (s *Service) UpdateMemberRole(ctx context.Context, teamID, userID, role string) error {
	member, err := s.repo.GetMember(ctx, teamID, userID)
	if err != nil {
		return err
	}

	if member.Source == SourceSSO {
		return errors.New("cannot update SSO-managed member role")
	}

	return s.repo.UpdateMemberRole(ctx, teamID, userID, role)
}

func (s *Service) ListMembers(ctx context.Context, teamID string) ([]*TeamMemberWithUser, error) {
	return s.repo.ListMembers(ctx, teamID)
}

func (s *Service) ListUserTeams(ctx context.Context, userID string) ([]*Team, error) {
	return s.repo.ListUserTeams(ctx, userID)
}

func (s *Service) ConvertMemberToManual(ctx context.Context, teamID, userID string) error {
	return s.repo.ConvertMemberToManual(ctx, teamID, userID)
}

func (s *Service) CreateSSOMapping(ctx context.Context, provider, ssoGroupName, teamID, memberRole string) (*SSOTeamMapping, error) {
	mapping := &SSOTeamMapping{
		Provider:     provider,
		SSOGroupName: ssoGroupName,
		TeamID:       teamID,
		MemberRole:   memberRole,
	}

	if err := s.repo.CreateSSOMapping(ctx, mapping); err != nil {
		return nil, err
	}

	return mapping, nil
}

func (s *Service) GetSSOMapping(ctx context.Context, id string) (*SSOTeamMapping, error) {
	return s.repo.GetSSOMapping(ctx, id)
}

func (s *Service) UpdateSSOMapping(ctx context.Context, id, teamID, memberRole string) error {
	return s.repo.UpdateSSOMapping(ctx, id, teamID, memberRole)
}

func (s *Service) DeleteSSOMapping(ctx context.Context, id string) error {
	return s.repo.DeleteSSOMapping(ctx, id)
}

func (s *Service) ListSSOMappings(ctx context.Context, provider string) ([]*SSOTeamMapping, error) {
	return s.repo.ListSSOMappings(ctx, provider)
}

// matchesGroupFilter checks if a group name matches the configured filter
func matchesGroupFilter(groupName string, filter config.TeamGroupFilter) bool {
	switch filter.Mode {
	case "none", "":
		return true
	case "prefix":
		return strings.HasPrefix(groupName, filter.Pattern)
	case "regex":
		if filter.Pattern == "" {
			return true
		}
		matched, err := regexp.MatchString(filter.Pattern, groupName)
		if err != nil {
			return false
		}
		return matched
	case "allowlist":
		if filter.Pattern == "" {
			return true
		}
		allowedGroups := strings.Split(filter.Pattern, ",")
		for _, allowed := range allowedGroups {
			if strings.TrimSpace(allowed) == groupName {
				return true
			}
		}
		return false
	case "denylist":
		if filter.Pattern == "" {
			return true
		}
		deniedGroups := strings.Split(filter.Pattern, ",")
		for _, denied := range deniedGroups {
			if strings.TrimSpace(denied) == groupName {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func getTeamNameForGroup(groupName string, stripPrefix string) string {
	if stripPrefix != "" && strings.HasPrefix(groupName, stripPrefix) {
		return strings.TrimPrefix(groupName, stripPrefix)
	}
	return groupName
}

func (s *Service) SyncUserTeamsFromSSO(ctx context.Context, userID, provider string, ssoGroups []string, syncConfig config.TeamSyncConfig) error {
	if len(ssoGroups) == 0 {
		return nil
	}

	mappings, err := s.repo.GetMappingsForGroups(ctx, provider, ssoGroups)
	if err != nil {
		return fmt.Errorf("failed to get mappings: %w", err)
	}

	mappedGroups := make(map[string]*SSOTeamMapping)
	for _, mapping := range mappings {
		mappedGroups[mapping.SSOGroupName] = mapping
	}

	if syncConfig.Enabled {
		for _, groupName := range ssoGroups {
			if _, exists := mappedGroups[groupName]; exists {
				continue
			}

			if !matchesGroupFilter(groupName, syncConfig.Group.Filter) {
				continue
			}

			teamName := getTeamNameForGroup(groupName, syncConfig.StripPrefix)

			existingTeam, err := s.repo.GetTeamByName(ctx, teamName)
			if err != nil && !errors.Is(err, ErrTeamNotFound) {
				return fmt.Errorf("failed to check for existing team: %w", err)
			}

			var teamID string
			if existingTeam != nil {
				teamID = existingTeam.ID
			} else {
				team, err := s.CreateTeamViaSSO(ctx, provider, teamName)
				if err != nil {
					return fmt.Errorf("failed to create team via SSO: %w", err)
				}
				teamID = team.ID
			}

			mapping, err := s.CreateSSOMapping(ctx, provider, groupName, teamID, RoleMember)
			if err != nil {
				return fmt.Errorf("failed to create SSO mapping: %w", err)
			}

			mappings = append(mappings, mapping)
			mappedGroups[groupName] = mapping
		}
	}

	userTeams, err := s.repo.ListUserTeams(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to list user teams: %w", err)
	}

	currentTeamMap := make(map[string]*Team)
	for _, team := range userTeams {
		currentTeamMap[team.ID] = team
	}

	for _, mapping := range mappings {
		member, err := s.repo.GetMember(ctx, mapping.TeamID, userID)

		if err != nil && !errors.Is(err, ErrMemberNotFound) {
			return fmt.Errorf("failed to get member: %w", err)
		}

		if member == nil {
			if err := s.repo.AddMember(ctx, &TeamMember{
				TeamID:      mapping.TeamID,
				UserID:      userID,
				Role:        mapping.MemberRole,
				Source:      SourceSSO,
				SSOProvider: &provider,
			}); err != nil && !errors.Is(err, ErrMemberAlreadyExists) {
				return fmt.Errorf("failed to add member: %w", err)
			}
		} else if member.Source == SourceSSO && member.Role != mapping.MemberRole {
			if err := s.repo.UpdateMemberRole(ctx, mapping.TeamID, userID, mapping.MemberRole); err != nil {
				return fmt.Errorf("failed to update member role: %w", err)
			}
		}
	}

	mappedTeamIDs := make(map[string]bool)
	for _, mapping := range mappings {
		mappedTeamIDs[mapping.TeamID] = true
	}

	for teamID := range currentTeamMap {
		if !mappedTeamIDs[teamID] {
			member, err := s.repo.GetMember(ctx, teamID, userID)
			if err != nil {
				continue
			}

			if member.Source == SourceSSO && member.SSOProvider != nil && *member.SSOProvider == provider {
				if err := s.repo.RemoveMember(ctx, teamID, userID); err != nil {
					return fmt.Errorf("failed to remove member: %w", err)
				}
			}
		}
	}

	return nil
}

func (s *Service) CreateTeamViaSSO(ctx context.Context, provider, groupName string) (*Team, error) {
	team := &Team{
		Name:          groupName,
		Description:   fmt.Sprintf("Auto-created from SSO provider: %s", provider),
		CreatedViaSSO: true,
		SSOProvider:   &provider,
	}

	if err := s.repo.CreateTeam(ctx, team); err != nil {
		return nil, err
	}

	return team, nil
}

func (s *Service) AddAssetOwner(ctx context.Context, assetID, ownerType, ownerID string) error {
	if ownerType != OwnerTypeUser && ownerType != OwnerTypeTeam {
		return errors.New("invalid owner type")
	}

	if ownerType == OwnerTypeTeam {
		if exists, err := s.repo.TeamExists(ctx, ownerID); err != nil {
			return err
		} else if !exists {
			return ErrTeamNotFound
		}
	}

	return s.repo.AddAssetOwner(ctx, assetID, ownerType, ownerID)
}

func (s *Service) RemoveAssetOwner(ctx context.Context, assetID, ownerType, ownerID string) error {
	return s.repo.RemoveAssetOwner(ctx, assetID, ownerType, ownerID)
}

func (s *Service) ListAssetOwners(ctx context.Context, assetID string) ([]*Owner, error) {
	return s.repo.ListAssetOwners(ctx, assetID)
}

func (s *Service) ListAssetsByOwner(ctx context.Context, ownerType, ownerID string) ([]string, error) {
	return s.repo.ListAssetsByOwner(ctx, ownerType, ownerID)
}

func (s *Service) CanUserAccessAsset(ctx context.Context, userID, assetID string) (bool, error) {
	owners, err := s.repo.ListAssetOwners(ctx, assetID)
	if err != nil {
		return false, err
	}

	for _, owner := range owners {
		if owner.Type == OwnerTypeUser && owner.ID == userID {
			return true, nil
		}

		if owner.Type == OwnerTypeTeam {
			isMember, err := s.repo.IsUserInTeam(ctx, userID, owner.ID)
			if err != nil {
				return false, err
			}
			if isMember {
				return true, nil
			}
		}
	}

	return false, nil
}

func (s *Service) SearchOwners(ctx context.Context, query string, limit int) ([]*Owner, error) {
	return s.repo.SearchOwners(ctx, query, limit)
}
