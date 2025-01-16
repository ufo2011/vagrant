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
	models = append(models, &Target{})
}

type Target struct {
	Model

	Configuration *ProtoValue
	Jobs          []*InternalJob `gorm:"polymorphic:Scope;" mapstructure:"-"`
	Metadata      MetadataSet
	Name          string  `gorm:"uniqueIndex:idx_pname;not null"`
	Parent        *Target `gorm:"foreignkey:ID"`
	ParentID      *uint   `mapstructure:"-"`
	Project       *Project
	ProjectID     uint `gorm:"uniqueIndex:idx_pname" mapstructure:"-"`
	Provider      *string
	Record        *ProtoValue
	ResourceId    string `gorm:"<-:create;uniqueIndex;not null"`
	State         vagrant_server.Operation_PhysicalState
	Subtargets    []*Target `gorm:"foreignkey:ParentID;constraint:OnDelete:SET NULL"`
	Uuid          *string   `gorm:"uniqueIndex"`
}

func (t *Target) find(db *gorm.DB) (*Target, error) {
	var target Target
	result := db.Preload(clause.Associations).
		Where(&Target{ResourceId: t.ResourceId}).
		Or(&Target{Uuid: t.Uuid}).
		Or(&Target{ProjectID: t.ProjectID, Name: t.Name}).
		Or(&Target{Model: Model{ID: t.ID}}).
		First(&target)
	if result.Error != nil {
		return nil, result.Error
	}

	return &target, nil
}

func (t *Target) scope() interface{} {
	return t
}

// Use before delete hook to remove all associations
func (t *Target) BeforeDelete(tx *gorm.DB) error {
	target, err := t.find(tx)
	if err != nil {
		return err
	}

	if len(target.Subtargets) > 0 {
		if result := tx.Delete(target.Subtargets); result.Error != nil {
			return result.Error
		}
	}

	return nil
}

// Set a public ID on the target before creating
func (t *Target) BeforeSave(tx *gorm.DB) error {
	if t.ResourceId == "" {
		if err := t.setId(); err != nil {
			return err
		}
	}

	if err := t.validate(tx); err != nil {
		return err
	}

	return nil
}

// NOTE: Need better validation on parent <-> subtarget
// project matching. It currently does basic check but
// will miss edge cases easily.
func (t *Target) validate(tx *gorm.DB) error {
	projectID := t.ProjectID
	if t.Project != nil {
		projectID = t.Project.ID
	}

	parent := &Target{}
	parentProjectID := uint(0)
	if t.Parent != nil {
		parent = t.Parent
	} else if t.ParentID != nil {
		result := tx.First(parent, &Target{Model: Model{ID: *t.ParentID}})
		if result.Error != nil {
			return result.Error
		}
	}
	if parent != nil {
		parentProjectID = parent.ProjectID
		if parent.Project != nil {
			parentProjectID = parent.Project.ID
		}
	}

	err := validation.ValidateStruct(t,
		validation.Field(&t.Name,
			validation.Required,
			validation.When(
				t.ID != 0,
				validation.By(
					checkUnique(
						tx.Model(&Target{}).
							Where(&Target{Name: t.Name, ProjectID: projectID}).
							Not(&Target{Model: Model{ID: t.ID}}),
					),
				),
			),
			validation.When(
				t.ID == 0,
				validation.By(
					checkUnique(
						tx.Model(&Target{}).
							Where(&Target{Name: t.Name, ProjectID: projectID}),
					),
				),
			),
		),
		validation.Field(&t.ResourceId,
			validation.Required,
			validation.When(
				t.ID == 0,
				validation.By(
					checkUnique(
						tx.Model(&Target{}).
							Where(&Target{ResourceId: t.ResourceId}),
					),
				),
			),
			validation.When(
				t.ID != 0,
				validation.By(
					checkNotModified(
						tx.Statement.Changed("ResourceId"),
					),
				),
			),
		),
		validation.Field(&t.Uuid,
			validation.When(t.Uuid != nil && t.ID != 0,
				validation.By(
					checkUnique(
						tx.Model(&Target{}).
							Where(&Target{Uuid: t.Uuid}).
							Not(&Target{Model: Model{ID: t.ID}}),
					),
				),
			),
			validation.When(t.Uuid != nil && t.ID == 0,
				validation.By(
					checkUnique(
						tx.Model(&Target{}).
							Where(&Target{Uuid: t.Uuid}),
					),
				),
			),
		),
		validation.Field(&t.ProjectID,
			validation.Required.When(t.Project == nil),
			validation.When(
				t.ProjectID != 0 && parentProjectID != 0,
				validation.By(
					checkSameProject(parentProjectID),
				),
			),
		),
		validation.Field(&t.Project,
			validation.Required.When(t.ProjectID == 0),
			validation.When(
				t.Project != nil && parentProjectID != 0,
				validation.By(
					checkSameProject(parentProjectID),
				),
			),
		),
	)

	if err != nil {
		return err
	}

	return nil
}

func (t *Target) setId() error {
	id, err := server.Id()
	if err != nil {
		return err
	}
	t.ResourceId = id

	return nil
}

// Convert target to reference protobuf message
func (t *Target) ToProtoRef() *vagrant_plugin_sdk.Ref_Target {
	if t == nil {
		return nil
	}

	var ref vagrant_plugin_sdk.Ref_Target

	err := decode(t, &ref)
	if err != nil {
		panic("failed to decode target to ref: " + err.Error())
	}

	return &ref
}

// Convert target to protobuf message
func (t *Target) ToProto() *vagrant_server.Target {
	if t == nil {
		return nil
	}

	var target vagrant_server.Target

	err := decode(t, &target)
	if err != nil {
		panic("failed to decode target: " + err.Error())
	}

	return &target
}

// Load a Target from reference protobuf message
func (s *State) TargetFromProtoRef(
	ref *vagrant_plugin_sdk.Ref_Target,
) (*Target, error) {
	if ref == nil {
		return nil, ErrEmptyProtoArgument
	}

	if ref.ResourceId == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var target Target
	result := s.search().Preload("Project.Basis").First(&target,
		&Target{ResourceId: ref.ResourceId},
	)
	if result.Error != nil {
		return nil, result.Error
	}

	return &target, nil
}

func (s *State) TargetFromProtoRefFuzzy(
	ref *vagrant_plugin_sdk.Ref_Target,
) (*Target, error) {
	target, err := s.TargetFromProtoRef(ref)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if ref.Project == nil {
		return nil, ErrMissingProtoParent
	}

	if ref.Name == "" {
		return nil, gorm.ErrRecordNotFound
	}

	target = &Target{}
	result := s.search().
		Joins("Project", &Project{ResourceId: ref.Project.ResourceId}).
		Preload("Project.Basis").
		First(target, &Target{Name: ref.Name})

	if result.Error != nil {
		return nil, result.Error
	}

	return target, nil
}

// Load a Target from protobuf message
func (s *State) TargetFromProto(
	t *vagrant_server.Target,
) (*Target, error) {
	target, err := s.TargetFromProtoRef(
		&vagrant_plugin_sdk.Ref_Target{
			ResourceId: t.ResourceId,
		},
	)

	if err != nil {
		return nil, err
	}

	return target, nil
}

func (s *State) TargetFromProtoFuzzy(
	t *vagrant_server.Target,
) (*Target, error) {
	target, err := s.TargetFromProto(t)
	if err == nil {
		return target, nil
	}

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if t.Uuid == "" && t.Name == "" {
		return nil, gorm.ErrRecordNotFound
	}

	if t.Project == nil && t.Uuid == "" {
		return nil, ErrMissingProtoParent
	}

	target = &Target{}
	if t.Project != nil {
		tx := s.search().
			Joins("Project").
			Preload("Project.Basis").
			Where("Project.resource_id = ?", t.Project.ResourceId)

		result := tx.First(target, &Target{Name: t.Name})
		if result.Error != nil {
			return nil, result.Error
		}

		return target, nil
	}

	tx := s.search().Preload("Project.Basis").
		Where("uuid LIKE ?", fmt.Sprintf("%%%s%%", t.Uuid))

	result := tx.First(target)
	if result.Error != nil {
		return nil, result.Error
	}

	return target, nil
}

// Get a target record using a reference protobuf message
func (s *State) TargetGet(
	ref *vagrant_plugin_sdk.Ref_Target,
) (*vagrant_server.Target, error) {
	t, err := s.TargetFromProtoRef(ref)
	if err != nil {
		return nil, lookupErrorToStatus("target", err)
	}

	return t.ToProto(), nil
}

// List all target records
func (s *State) TargetList() ([]*vagrant_plugin_sdk.Ref_Target, error) {
	var targets []Target
	result := s.search().Find(&targets)
	if result.Error != nil {
		return nil, lookupErrorToStatus("targets", result.Error)
	}

	trefs := make([]*vagrant_plugin_sdk.Ref_Target, len(targets))
	for i, t := range targets {
		trefs[i] = t.ToProtoRef()
	}

	return trefs, nil
}

// Delete a target by reference protobuf message
func (s *State) TargetDelete(
	t *vagrant_plugin_sdk.Ref_Target,
) error {
	target, err := s.TargetFromProtoRef(t)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}

	if err != nil {
		return lookupErrorToStatus("target", err)
	}

	result := s.db.Delete(target)
	if result.Error != nil {
		return deleteErrorToStatus("target", result.Error)
	}

	return nil
}

// Store a Target
func (s *State) TargetPut(
	t *vagrant_server.Target,
) (*vagrant_server.Target, error) {
	target, err := s.TargetFromProtoFuzzy(t)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, lookupErrorToStatus("target", err)
	}

	// If a target is found, remove the project
	// ref to prevent update attempts
	if target != nil {
		t.Project = nil
	} else {
		// Otherwise, init target for the decode
		target = &Target{}
	}

	// TODO(spox): forcing the record to be updated here
	// but the soft decoding should be handling it properly
	if t.Record != nil {
		target.Record = nil
	}

	err = s.softDecode(t, target)
	if err != nil {
		return nil, saveErrorToStatus("target", err)
	}

	if target.Project == nil {
		return nil, saveErrorToStatus("target", ErrMissingProtoParent)
	}

	// NOTE: this does not get set when updating to
	//       the UNKNOWN value since it's a zero
	//       value and thus ignored with soft decode
	target.State = t.State

	if err := s.upsertFull(target); err != nil {
		return nil, saveErrorToStatus("target", err)
	}

	s.log.Info("target has been upserted", "record", target.Record, "original-record", t.Record)

	return target.ToProto(), nil
}

// Find a Target
func (s *State) TargetFind(
	t *vagrant_server.Target,
) (*vagrant_server.Target, error) {
	target, err := s.TargetFromProtoFuzzy(t)
	if err != nil {
		return nil, lookupErrorToStatus("target", err)
	}

	return target.ToProto(), nil
}

var (
	_ scope = (*Target)(nil)
)
