// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"errors"
	"fmt"

	"github.com/go-ozzo/ozzo-validation/v4"
	"github.com/hashicorp/vagrant-plugin-sdk/proto/vagrant_plugin_sdk"
	"github.com/hashicorp/vagrant/internal/server"
	"github.com/hashicorp/vagrant/internal/server/proto/vagrant_server"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	models = append(models, &Project{})
}

type Project struct {
	Model

	Basis         *Basis       `gorm:"constraint:OnDelete:SET NULL"`
	BasisID       uint         `gorm:"uniqueIndex:idx_bname" mapstructure:"-"`
	Vagrantfile   *Vagrantfile `gorm:"constraint:OnDelete:SET NULL" mapstructure:"Configuration"`
	VagrantfileID *uint        `mapstructure:"-" gorm:"constraint:OnDelete:SET NULL"`
	DataSource    *ProtoValue
	Jobs          []*InternalJob `gorm:"polymorphic:Scope"`
	Metadata      MetadataSet
	Name          string `gorm:"uniqueIndex:idx_bname;not null"`
	Path          string `gorm:"uniqueIndex:idx_bname;not null"`
	RemoteEnabled bool
	ResourceId    string `gorm:"<-:create;uniqueIndex;not null"`
	Targets       []*Target
}

func (p *Project) scope() interface{} {
	return p
}

func (p *Project) find(db *gorm.DB) (*Project, error) {
	var project Project
	result := db.Preload(clause.Associations).
		Where(&Project{ResourceId: p.ResourceId}).
		Or(&Project{BasisID: p.BasisID, Name: p.Name}).
		Or(&Project{BasisID: p.BasisID, Path: p.Path}).
		Or(&Project{Model: Model{ID: p.ID}}).
		First(&project)
	if result.Error != nil {
		return nil, result.Error
	}

	return &project, nil
}

// Use before delete hook to remove all assocations
func (p *Project) BeforeDelete(tx *gorm.DB) error {
	project, err := p.find(tx)
	if err != nil {
		return err
	}

	if project.VagrantfileID != nil {
		result := tx.Where(&Vagrantfile{Model: Model{ID: *project.VagrantfileID}}).
			Delete(&Vagrantfile{})
		if result.Error != nil {
			return result.Error
		}
	}

	if len(project.Targets) > 0 {
		if result := tx.Delete(project.Targets); result.Error != nil {
			return result.Error
		}
	}

	if len(project.Jobs) > 0 {
		if result := tx.Delete(project.Jobs); result.Error != nil {
			return result.Error
		}
	}

	return nil
}

// Set a public ID on the project before creating
func (p *Project) BeforeSave(tx *gorm.DB) error {
	if p.ResourceId == "" {
		if err := p.setId(); err != nil {
			return err
		}
	}

	if err := p.Validate(tx); err != nil {
		return err
	}

	return nil
}

func (p *Project) BeforeUpdate(tx *gorm.DB) error {
	// If a Vagrantfile was already set for the project, just update it
	if p.Vagrantfile != nil && p.Vagrantfile.ID == 0 && p.VagrantfileID != nil {
		var v Vagrantfile
		result := tx.First(&v, &Vagrantfile{Model: Model{ID: *p.VagrantfileID}})
		if result.Error != nil {
			return result.Error
		}
		id := v.ID
		if err := decode(p.Vagrantfile, &v); err != nil {
			return err
		}
		v.ID = id
		p.Vagrantfile = &v

		// NOTE: Just updating the value doesn't save the changes so
		//       save the changes in this transaction
		if result := tx.Save(&v); result.Error != nil {
			return result.Error
		}
	}
	return nil
}

func (p *Project) Validate(tx *gorm.DB) error {
	basisID := p.BasisID
	if p.Basis != nil {
		basisID = p.Basis.ID
	}

	err := validation.ValidateStruct(p,
		validation.Field(&p.BasisID,
			validation.Required.When(p.Basis == nil),
		),
		validation.Field(&p.Basis,
			validation.Required.When(p.BasisID == 0),
		),
		validation.Field(&p.Name,
			validation.Required,
			validation.When(
				p.ID != 0,
				validation.By(
					checkUnique(
						tx.Model(&Project{}).
							Where(&Project{Name: p.Name, BasisID: basisID}).
							Not(&Project{Model: Model{ID: p.ID}}),
					),
				),
			),
			validation.When(
				p.ID == 0,
				validation.By(
					checkUnique(
						tx.Model(&Project{}).
							Where(&Project{Name: p.Name, BasisID: basisID}),
					),
				),
			),
		),
		validation.Field(&p.Path,
			validation.Required,
			validation.When(
				p.ID != 0,
				validation.By(
					checkUnique(
						tx.Model(&Project{}).
							Where(&Project{Path: p.Path, BasisID: basisID}).
							Not(&Project{Model: Model{ID: p.ID}}),
					),
				),
			),
			validation.When(
				p.ID == 0,
				validation.By(
					checkUnique(
						tx.Model(&Project{}).
							Where(&Project{Path: p.Path, BasisID: basisID}),
					),
				),
			),
		),
		validation.Field(&p.ResourceId,
			validation.Required,
			validation.When(
				p.ID == 0,
				validation.By(
					checkUnique(
						tx.Model(&Project{}).
							Where(&Project{ResourceId: p.ResourceId}),
					),
				),
			),
			validation.When(
				p.ID != 0,
				validation.By(
					checkNotModified(
						tx.Statement.Changed("ResourceId"),
					),
				),
			),
		),
	)

	if err != nil {
		return err
	}

	return nil
}

func (p *Project) setId() error {
	id, err := server.Id()
	if err != nil {
		return err
	}
	p.ResourceId = id

	return nil
}

// Convert project to reference protobuf message
func (p *Project) ToProtoRef() *vagrant_plugin_sdk.Ref_Project {
	if p == nil {
		return nil
	}

	ref := vagrant_plugin_sdk.Ref_Project{}
	err := decode(p, &ref)
	if err != nil {
		panic("failed to decode project to ref: " + err.Error())
	}

	return &ref
}

// Convert project to protobuf message
func (p *Project) ToProto() *vagrant_server.Project {
	if p == nil {
		return nil
	}
	var project vagrant_server.Project

	err := decode(p, &project)
	if err != nil {
		panic("failed to decode project: " + err.Error())
	}

	// Manually include the vagrantfile since we force it to be ignored
	if p.Vagrantfile != nil {
		project.Configuration = p.Vagrantfile.ToProto()
	}

	return &project
}

// Load a Project from reference protobuf message.
func (s *State) ProjectFromProtoRef(
	ref *vagrant_plugin_sdk.Ref_Project,
) (*Project, error) {
	if ref == nil {
		return nil, ErrEmptyProtoArgument
	}

	if ref.ResourceId == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var project Project
	result := s.search().First(&project,
		&Project{ResourceId: ref.ResourceId})
	if result.Error != nil {
		return nil, result.Error
	}

	return &project, nil
}

func (s *State) ProjectFromProtoRefFuzzy(
	ref *vagrant_plugin_sdk.Ref_Project,
) (*Project, error) {
	project, err := s.ProjectFromProtoRef(ref)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if ref.Basis == nil {
		return nil, ErrMissingProtoParent
	}

	if ref.Name == "" && ref.Path == "" {
		return nil, gorm.ErrRecordNotFound
	}

	project = &Project{}
	query := &Project{}

	if ref.Name != "" {
		query.Name = ref.Name
	}
	if ref.Path != "" {
		query.Path = ref.Path
	}

	result := s.search().
		Joins("Basis", &Basis{ResourceId: ref.Basis.ResourceId}).
		Where(query).
		First(project)

	if result.Error != nil {
		return nil, result.Error
	}

	return project, nil
}

// Load a Project from protobuf message.
func (s *State) ProjectFromProto(
	p *vagrant_server.Project,
) (*Project, error) {
	if p == nil {
		return nil, ErrEmptyProtoArgument
	}

	project, err := s.ProjectFromProtoRef(
		&vagrant_plugin_sdk.Ref_Project{
			ResourceId: p.ResourceId,
		},
	)

	if err != nil {
		return nil, err
	}

	return project, nil
}

func (s *State) ProjectFromProtoFuzzy(
	p *vagrant_server.Project,
) (*Project, error) {
	if p == nil {
		return nil, ErrEmptyProtoArgument
	}

	project, err := s.ProjectFromProtoRefFuzzy(
		&vagrant_plugin_sdk.Ref_Project{
			ResourceId: p.ResourceId,
			Basis:      p.Basis,
			Name:       p.Name,
			Path:       p.Path,
		},
	)
	if err != nil {
		return nil, err
	}

	return project, nil
}

// Get a project record using a reference protobuf message
func (s *State) ProjectGet(
	p *vagrant_plugin_sdk.Ref_Project,
) (*vagrant_server.Project, error) {
	project, err := s.ProjectFromProtoRef(p)
	if err != nil {
		return nil, lookupErrorToStatus("project", err)
	}

	return project.ToProto(), nil
}

// Store a Project
func (s *State) ProjectPut(
	p *vagrant_server.Project,
) (*vagrant_server.Project, error) {
	project, err := s.ProjectFromProto(p)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, lookupErrorToStatus("project", err)
	}

	// If a project is found, remove the basis
	// ref to prevent update attempts
	if project != nil {
		p.Basis = nil
	} else {
		project = &Project{}
	}

	err = s.softDecode(p, project)
	if err != nil {
		return nil, saveErrorToStatus("project", err)
	}

	if p.Configuration != nil && p.Configuration.Finalized == nil {
		project.Vagrantfile.Finalized = nil
	}

	if err := s.upsertFull(project); err != nil {
		return nil, saveErrorToStatus("project", err)
	}

	return project.ToProto(), nil
}

// List all project records
func (s *State) ProjectList() ([]*vagrant_plugin_sdk.Ref_Project, error) {
	var projects []Project
	result := s.search().Find(&projects)
	if result.Error != nil {
		return nil, lookupErrorToStatus("projects", result.Error)
	}

	prefs := make([]*vagrant_plugin_sdk.Ref_Project, len(projects))
	for i, prj := range projects {
		prefs[i] = prj.ToProtoRef()
	}

	return prefs, nil
}

// Find a Project using a protobuf message
func (s *State) ProjectFind(p *vagrant_server.Project) (*vagrant_server.Project, error) {
	project, err := s.ProjectFromProtoFuzzy(p)
	if err != nil {
		return nil, lookupErrorToStatus("project", fmt.Errorf("%w (%#v)", err, p))
	}

	return project.ToProto(), nil
}

// Delete a project
func (s *State) ProjectDelete(
	p *vagrant_plugin_sdk.Ref_Project,
) error {
	project, err := s.ProjectFromProtoRef(p)
	// If the record was not found, we return with no error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}

	// If an unexpected error was encountered, return it
	if err != nil {
		return deleteErrorToStatus("project", err)
	}

	result := s.db.Delete(project)
	if result.Error != nil {
		return deleteErrorToStatus("project", err)
	}

	return nil
}

var (
	_ scope = (*Project)(nil)
)
