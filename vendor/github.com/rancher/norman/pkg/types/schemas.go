package types

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/rancher/norman/pkg/types/convert"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/rancher/wrangler/pkg/name"
)

type SchemaCollection struct {
	Data []Schema
}

type SchemasInitFunc func(*Schemas) *Schemas

type MapperFactory func() Mapper

type FieldMapperFactory func(fieldName string, args ...string) Mapper

type Schemas struct {
	sync.Mutex
	processingTypes   map[reflect.Type]*Schema
	typeNames         map[reflect.Type]string
	schemasByID       map[string]*Schema
	mappers           map[string][]Mapper
	embedded          map[string]*Schema
	fieldMappers      map[string]FieldMapperFactory
	DefaultMapper     MapperFactory
	DefaultPostMapper MapperFactory
	schemas           []*Schema
}

func EmptySchemas() *Schemas {
	s, _ := NewSchemas()
	return s
}

func NewSchemas(schemas ...*Schemas) (*Schemas, error) {
	var (
		errs []error
	)

	s := &Schemas{
		processingTypes: map[reflect.Type]*Schema{},
		typeNames:       map[reflect.Type]string{},
		schemasByID:     map[string]*Schema{},
		mappers:         map[string][]Mapper{},
		embedded:        map[string]*Schema{},
	}

	for _, schemas := range schemas {
		if _, err := s.AddSchemas(schemas); err != nil {
			errs = append(errs, err)
		}
	}

	return s, merr.NewErrors(errs...)
}

func (s *Schemas) Init(initFunc SchemasInitFunc) *Schemas {
	return initFunc(s)
}

func (s *Schemas) AddSchemas(schema *Schemas) (*Schemas, error) {
	var errs []error
	for _, schema := range schema.Schemas() {
		if err := s.AddSchema(*schema); err != nil {
			errs = append(errs, err)
		}
	}
	return s, merr.NewErrors(errs...)
}

func (s *Schemas) RemoveSchema(schema Schema) *Schemas {
	s.Lock()
	defer s.Unlock()
	return s.doRemoveSchema(schema)
}

func (s *Schemas) doRemoveSchema(schema Schema) *Schemas {
	delete(s.schemasByID, schema.ID)
	return s
}

func (s *Schemas) MustAddSchema(schema Schema) *Schemas {
	err := s.AddSchema(schema)
	if err != nil {
		panic(err)
	}
	return s
}

func (s *Schemas) AddSchema(schema Schema) error {
	s.Lock()
	defer s.Unlock()
	return s.doAddSchema(schema)
}

func (s *Schemas) doAddSchema(schema Schema) error {
	if err := s.setupDefaults(&schema); err != nil {
		return err
	}

	existing, ok := s.schemasByID[schema.ID]
	if ok {
		*existing = schema
	} else {
		s.schemasByID[schema.ID] = &schema
		s.schemas = append(s.schemas, &schema)
	}

	return nil
}

func (s *Schemas) setupDefaults(schema *Schema) (err error) {
	schema.Type = "schema"
	if schema.ID == "" {
		return fmt.Errorf("ID is not set on schema: %v", schema)
	}
	if schema.PluralName == "" {
		schema.PluralName = name.GuessPluralName(schema.ID)
	}
	if schema.CodeName == "" {
		schema.CodeName = convert.Capitalize(schema.ID)
	}
	if schema.CodeNamePlural == "" {
		schema.CodeNamePlural = name.GuessPluralName(schema.CodeName)
	}
	if err := s.assignMappers(schema); err != nil {
		return err
	}

	return
}

func (s *Schemas) AddFieldMapper(name string, factory FieldMapperFactory) *Schemas {
	if s.fieldMappers == nil {
		s.fieldMappers = map[string]FieldMapperFactory{}
	}
	s.fieldMappers[name] = factory
	return s
}

func (s *Schemas) AddMapper(schemaID string, mapper Mapper) *Schemas {
	s.mappers[schemaID] = append(s.mappers[schemaID], mapper)
	return s
}

func (s *Schemas) Schemas() []*Schema {
	return s.schemas
}

func (s *Schemas) SchemasByID() map[string]*Schema {
	return s.schemasByID
}

func (s *Schemas) mapper(schemaID string) []Mapper {
	return s.mappers[schemaID]
}

func (s *Schemas) Schema(name string) *Schema {
	return s.doSchema(name, true)
}

func (s *Schemas) doSchema(name string, lock bool) *Schema {
	if lock {
		s.Lock()
	}
	schema, ok := s.schemasByID[name]
	if lock {
		s.Unlock()
	}
	if ok {
		return schema
	}

	for _, check := range s.schemas {
		if strings.EqualFold(check.ID, name) || strings.EqualFold(check.PluralName, name) {
			return check
		}
	}

	return nil
}

type MultiErrors struct {
	Errors []error
}

type Errors struct {
	errors []error
}

func (e *Errors) Add(err error) {
	if err != nil {
		e.errors = append(e.errors, err)
	}
}

func (e *Errors) Err() error {
	return NewErrors(e.errors...)
}

func NewErrors(inErrors ...error) error {
	var errors []error
	for _, err := range inErrors {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) == 0 {
		return nil
	} else if len(errors) == 1 {
		return errors[0]
	}
	return &MultiErrors{
		Errors: errors,
	}
}

func (m *MultiErrors) Error() string {
	buf := bytes.NewBuffer(nil)
	for _, err := range m.Errors {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(err.Error())
	}

	return buf.String()
}