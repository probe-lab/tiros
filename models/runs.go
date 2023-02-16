// Code generated by SQLBoiler 4.14.1 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/friendsofgo/errors"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"github.com/volatiletech/sqlboiler/v4/queries/qmhelper"
	"github.com/volatiletech/sqlboiler/v4/types"
	"github.com/volatiletech/strmangle"
)

// Run is an object representing the database table.
type Run struct {
	ID              int               `boil:"id" json:"id" toml:"id" yaml:"id"`
	Regions         types.StringArray `boil:"regions" json:"regions" toml:"regions" yaml:"regions"`
	Urls            types.StringArray `boil:"urls" json:"urls" toml:"urls" yaml:"urls"`
	SettleShort     float64           `boil:"settle_short" json:"settle_short" toml:"settle_short" yaml:"settle_short"`
	SettleLong      float64           `boil:"settle_long" json:"settle_long" toml:"settle_long" yaml:"settle_long"`
	NodesPerVersion int16             `boil:"nodes_per_version" json:"nodes_per_version" toml:"nodes_per_version" yaml:"nodes_per_version"`
	Versions        types.StringArray `boil:"versions" json:"versions" toml:"versions" yaml:"versions"`
	Times           int16             `boil:"times" json:"times" toml:"times" yaml:"times"`
	UpdatedAt       time.Time         `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`
	CreatedAt       time.Time         `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`

	R *runR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L runL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

var RunColumns = struct {
	ID              string
	Regions         string
	Urls            string
	SettleShort     string
	SettleLong      string
	NodesPerVersion string
	Versions        string
	Times           string
	UpdatedAt       string
	CreatedAt       string
}{
	ID:              "id",
	Regions:         "regions",
	Urls:            "urls",
	SettleShort:     "settle_short",
	SettleLong:      "settle_long",
	NodesPerVersion: "nodes_per_version",
	Versions:        "versions",
	Times:           "times",
	UpdatedAt:       "updated_at",
	CreatedAt:       "created_at",
}

var RunTableColumns = struct {
	ID              string
	Regions         string
	Urls            string
	SettleShort     string
	SettleLong      string
	NodesPerVersion string
	Versions        string
	Times           string
	UpdatedAt       string
	CreatedAt       string
}{
	ID:              "runs.id",
	Regions:         "runs.regions",
	Urls:            "runs.urls",
	SettleShort:     "runs.settle_short",
	SettleLong:      "runs.settle_long",
	NodesPerVersion: "runs.nodes_per_version",
	Versions:        "runs.versions",
	Times:           "runs.times",
	UpdatedAt:       "runs.updated_at",
	CreatedAt:       "runs.created_at",
}

// Generated where

type whereHelpertypes_StringArray struct{ field string }

func (w whereHelpertypes_StringArray) EQ(x types.StringArray) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.EQ, x)
}
func (w whereHelpertypes_StringArray) NEQ(x types.StringArray) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.NEQ, x)
}
func (w whereHelpertypes_StringArray) LT(x types.StringArray) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.LT, x)
}
func (w whereHelpertypes_StringArray) LTE(x types.StringArray) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.LTE, x)
}
func (w whereHelpertypes_StringArray) GT(x types.StringArray) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.GT, x)
}
func (w whereHelpertypes_StringArray) GTE(x types.StringArray) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.GTE, x)
}

type whereHelperfloat64 struct{ field string }

func (w whereHelperfloat64) EQ(x float64) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.EQ, x) }
func (w whereHelperfloat64) NEQ(x float64) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.NEQ, x)
}
func (w whereHelperfloat64) LT(x float64) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.LT, x) }
func (w whereHelperfloat64) LTE(x float64) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.LTE, x)
}
func (w whereHelperfloat64) GT(x float64) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.GT, x) }
func (w whereHelperfloat64) GTE(x float64) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.GTE, x)
}
func (w whereHelperfloat64) IN(slice []float64) qm.QueryMod {
	values := make([]interface{}, 0, len(slice))
	for _, value := range slice {
		values = append(values, value)
	}
	return qm.WhereIn(fmt.Sprintf("%s IN ?", w.field), values...)
}
func (w whereHelperfloat64) NIN(slice []float64) qm.QueryMod {
	values := make([]interface{}, 0, len(slice))
	for _, value := range slice {
		values = append(values, value)
	}
	return qm.WhereNotIn(fmt.Sprintf("%s NOT IN ?", w.field), values...)
}

var RunWhere = struct {
	ID              whereHelperint
	Regions         whereHelpertypes_StringArray
	Urls            whereHelpertypes_StringArray
	SettleShort     whereHelperfloat64
	SettleLong      whereHelperfloat64
	NodesPerVersion whereHelperint16
	Versions        whereHelpertypes_StringArray
	Times           whereHelperint16
	UpdatedAt       whereHelpertime_Time
	CreatedAt       whereHelpertime_Time
}{
	ID:              whereHelperint{field: "\"runs\".\"id\""},
	Regions:         whereHelpertypes_StringArray{field: "\"runs\".\"regions\""},
	Urls:            whereHelpertypes_StringArray{field: "\"runs\".\"urls\""},
	SettleShort:     whereHelperfloat64{field: "\"runs\".\"settle_short\""},
	SettleLong:      whereHelperfloat64{field: "\"runs\".\"settle_long\""},
	NodesPerVersion: whereHelperint16{field: "\"runs\".\"nodes_per_version\""},
	Versions:        whereHelpertypes_StringArray{field: "\"runs\".\"versions\""},
	Times:           whereHelperint16{field: "\"runs\".\"times\""},
	UpdatedAt:       whereHelpertime_Time{field: "\"runs\".\"updated_at\""},
	CreatedAt:       whereHelpertime_Time{field: "\"runs\".\"created_at\""},
}

// RunRels is where relationship names are stored.
var RunRels = struct {
	Measurements string
}{
	Measurements: "Measurements",
}

// runR is where relationships are stored.
type runR struct {
	Measurements MeasurementSlice `boil:"Measurements" json:"Measurements" toml:"Measurements" yaml:"Measurements"`
}

// NewStruct creates a new relationship struct
func (*runR) NewStruct() *runR {
	return &runR{}
}

func (r *runR) GetMeasurements() MeasurementSlice {
	if r == nil {
		return nil
	}
	return r.Measurements
}

// runL is where Load methods for each relationship are stored.
type runL struct{}

var (
	runAllColumns            = []string{"id", "regions", "urls", "settle_short", "settle_long", "nodes_per_version", "versions", "times", "updated_at", "created_at"}
	runColumnsWithoutDefault = []string{"regions", "urls", "settle_short", "settle_long", "nodes_per_version", "versions", "times", "updated_at", "created_at"}
	runColumnsWithDefault    = []string{"id"}
	runPrimaryKeyColumns     = []string{"id"}
	runGeneratedColumns      = []string{"id"}
)

type (
	// RunSlice is an alias for a slice of pointers to Run.
	// This should almost always be used instead of []Run.
	RunSlice []*Run
	// RunHook is the signature for custom Run hook methods
	RunHook func(context.Context, boil.ContextExecutor, *Run) error

	runQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	runType                 = reflect.TypeOf(&Run{})
	runMapping              = queries.MakeStructMapping(runType)
	runPrimaryKeyMapping, _ = queries.BindMapping(runType, runMapping, runPrimaryKeyColumns)
	runInsertCacheMut       sync.RWMutex
	runInsertCache          = make(map[string]insertCache)
	runUpdateCacheMut       sync.RWMutex
	runUpdateCache          = make(map[string]updateCache)
	runUpsertCacheMut       sync.RWMutex
	runUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force qmhelper dependency for where clause generation (which doesn't
	// always happen)
	_ = qmhelper.Where
)

var runAfterSelectHooks []RunHook

var runBeforeInsertHooks []RunHook
var runAfterInsertHooks []RunHook

var runBeforeUpdateHooks []RunHook
var runAfterUpdateHooks []RunHook

var runBeforeDeleteHooks []RunHook
var runAfterDeleteHooks []RunHook

var runBeforeUpsertHooks []RunHook
var runAfterUpsertHooks []RunHook

// doAfterSelectHooks executes all "after Select" hooks.
func (o *Run) doAfterSelectHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range runAfterSelectHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeInsertHooks executes all "before insert" hooks.
func (o *Run) doBeforeInsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range runBeforeInsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterInsertHooks executes all "after Insert" hooks.
func (o *Run) doAfterInsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range runAfterInsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpdateHooks executes all "before Update" hooks.
func (o *Run) doBeforeUpdateHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range runBeforeUpdateHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpdateHooks executes all "after Update" hooks.
func (o *Run) doAfterUpdateHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range runAfterUpdateHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeDeleteHooks executes all "before Delete" hooks.
func (o *Run) doBeforeDeleteHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range runBeforeDeleteHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterDeleteHooks executes all "after Delete" hooks.
func (o *Run) doAfterDeleteHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range runAfterDeleteHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpsertHooks executes all "before Upsert" hooks.
func (o *Run) doBeforeUpsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range runBeforeUpsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpsertHooks executes all "after Upsert" hooks.
func (o *Run) doAfterUpsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range runAfterUpsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// AddRunHook registers your hook function for all future operations.
func AddRunHook(hookPoint boil.HookPoint, runHook RunHook) {
	switch hookPoint {
	case boil.AfterSelectHook:
		runAfterSelectHooks = append(runAfterSelectHooks, runHook)
	case boil.BeforeInsertHook:
		runBeforeInsertHooks = append(runBeforeInsertHooks, runHook)
	case boil.AfterInsertHook:
		runAfterInsertHooks = append(runAfterInsertHooks, runHook)
	case boil.BeforeUpdateHook:
		runBeforeUpdateHooks = append(runBeforeUpdateHooks, runHook)
	case boil.AfterUpdateHook:
		runAfterUpdateHooks = append(runAfterUpdateHooks, runHook)
	case boil.BeforeDeleteHook:
		runBeforeDeleteHooks = append(runBeforeDeleteHooks, runHook)
	case boil.AfterDeleteHook:
		runAfterDeleteHooks = append(runAfterDeleteHooks, runHook)
	case boil.BeforeUpsertHook:
		runBeforeUpsertHooks = append(runBeforeUpsertHooks, runHook)
	case boil.AfterUpsertHook:
		runAfterUpsertHooks = append(runAfterUpsertHooks, runHook)
	}
}

// One returns a single run record from the query.
func (q runQuery) One(ctx context.Context, exec boil.ContextExecutor) (*Run, error) {
	o := &Run{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(ctx, exec, o)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for runs")
	}

	if err := o.doAfterSelectHooks(ctx, exec); err != nil {
		return o, err
	}

	return o, nil
}

// All returns all Run records from the query.
func (q runQuery) All(ctx context.Context, exec boil.ContextExecutor) (RunSlice, error) {
	var o []*Run

	err := q.Bind(ctx, exec, &o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to Run slice")
	}

	if len(runAfterSelectHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterSelectHooks(ctx, exec); err != nil {
				return o, err
			}
		}
	}

	return o, nil
}

// Count returns the count of all Run records in the query.
func (q runQuery) Count(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count runs rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table.
func (q runQuery) Exists(ctx context.Context, exec boil.ContextExecutor) (bool, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if runs exists")
	}

	return count > 0, nil
}

// Measurements retrieves all the measurement's Measurements with an executor.
func (o *Run) Measurements(mods ...qm.QueryMod) measurementQuery {
	var queryMods []qm.QueryMod
	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"measurements\".\"run_id\"=?", o.ID),
	)

	return Measurements(queryMods...)
}

// LoadMeasurements allows an eager lookup of values, cached into the
// loaded structs of the objects. This is for a 1-M or N-M relationship.
func (runL) LoadMeasurements(ctx context.Context, e boil.ContextExecutor, singular bool, maybeRun interface{}, mods queries.Applicator) error {
	var slice []*Run
	var object *Run

	if singular {
		var ok bool
		object, ok = maybeRun.(*Run)
		if !ok {
			object = new(Run)
			ok = queries.SetFromEmbeddedStruct(&object, &maybeRun)
			if !ok {
				return errors.New(fmt.Sprintf("failed to set %T from embedded struct %T", object, maybeRun))
			}
		}
	} else {
		s, ok := maybeRun.(*[]*Run)
		if ok {
			slice = *s
		} else {
			ok = queries.SetFromEmbeddedStruct(&slice, maybeRun)
			if !ok {
				return errors.New(fmt.Sprintf("failed to set %T from embedded struct %T", slice, maybeRun))
			}
		}
	}

	args := make([]interface{}, 0, 1)
	if singular {
		if object.R == nil {
			object.R = &runR{}
		}
		args = append(args, object.ID)
	} else {
	Outer:
		for _, obj := range slice {
			if obj.R == nil {
				obj.R = &runR{}
			}

			for _, a := range args {
				if a == obj.ID {
					continue Outer
				}
			}

			args = append(args, obj.ID)
		}
	}

	if len(args) == 0 {
		return nil
	}

	query := NewQuery(
		qm.From(`measurements`),
		qm.WhereIn(`measurements.run_id in ?`, args...),
	)
	if mods != nil {
		mods.Apply(query)
	}

	results, err := query.QueryContext(ctx, e)
	if err != nil {
		return errors.Wrap(err, "failed to eager load measurements")
	}

	var resultSlice []*Measurement
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice measurements")
	}

	if err = results.Close(); err != nil {
		return errors.Wrap(err, "failed to close results in eager load on measurements")
	}
	if err = results.Err(); err != nil {
		return errors.Wrap(err, "error occurred during iteration of eager loaded relations for measurements")
	}

	if len(measurementAfterSelectHooks) != 0 {
		for _, obj := range resultSlice {
			if err := obj.doAfterSelectHooks(ctx, e); err != nil {
				return err
			}
		}
	}
	if singular {
		object.R.Measurements = resultSlice
		for _, foreign := range resultSlice {
			if foreign.R == nil {
				foreign.R = &measurementR{}
			}
			foreign.R.Run = object
		}
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.RunID {
				local.R.Measurements = append(local.R.Measurements, foreign)
				if foreign.R == nil {
					foreign.R = &measurementR{}
				}
				foreign.R.Run = local
				break
			}
		}
	}

	return nil
}

// AddMeasurements adds the given related objects to the existing relationships
// of the run, optionally inserting them as new records.
// Appends related to o.R.Measurements.
// Sets related.R.Run appropriately.
func (o *Run) AddMeasurements(ctx context.Context, exec boil.ContextExecutor, insert bool, related ...*Measurement) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.RunID = o.ID
			if err = rel.Insert(ctx, exec, boil.Infer()); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"measurements\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"run_id"}),
				strmangle.WhereClause("\"", "\"", 2, measurementPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.IsDebug(ctx) {
				writer := boil.DebugWriterFrom(ctx)
				fmt.Fprintln(writer, updateQuery)
				fmt.Fprintln(writer, values)
			}
			if _, err = exec.ExecContext(ctx, updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.RunID = o.ID
		}
	}

	if o.R == nil {
		o.R = &runR{
			Measurements: related,
		}
	} else {
		o.R.Measurements = append(o.R.Measurements, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &measurementR{
				Run: o,
			}
		} else {
			rel.R.Run = o
		}
	}
	return nil
}

// Runs retrieves all the records using an executor.
func Runs(mods ...qm.QueryMod) runQuery {
	mods = append(mods, qm.From("\"runs\""))
	q := NewQuery(mods...)
	if len(queries.GetSelect(q)) == 0 {
		queries.SetSelect(q, []string{"\"runs\".*"})
	}

	return runQuery{q}
}

// FindRun retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindRun(ctx context.Context, exec boil.ContextExecutor, iD int, selectCols ...string) (*Run, error) {
	runObj := &Run{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"runs\" where \"id\"=$1", sel,
	)

	q := queries.Raw(query, iD)

	err := q.Bind(ctx, exec, runObj)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from runs")
	}

	if err = runObj.doAfterSelectHooks(ctx, exec); err != nil {
		return runObj, err
	}

	return runObj, nil
}

// Insert a single record using an executor.
// See boil.Columns.InsertColumnSet documentation to understand column list inference for inserts.
func (o *Run) Insert(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) error {
	if o == nil {
		return errors.New("models: no runs provided for insertion")
	}

	var err error
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		if o.UpdatedAt.IsZero() {
			o.UpdatedAt = currTime
		}
		if o.CreatedAt.IsZero() {
			o.CreatedAt = currTime
		}
	}

	if err := o.doBeforeInsertHooks(ctx, exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(runColumnsWithDefault, o)

	key := makeCacheKey(columns, nzDefaults)
	runInsertCacheMut.RLock()
	cache, cached := runInsertCache[key]
	runInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := columns.InsertColumnSet(
			runAllColumns,
			runColumnsWithDefault,
			runColumnsWithoutDefault,
			nzDefaults,
		)
		wl = strmangle.SetComplement(wl, runGeneratedColumns)

		cache.valueMapping, err = queries.BindMapping(runType, runMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(runType, runMapping, returnColumns)
		if err != nil {
			return err
		}
		if len(wl) != 0 {
			cache.query = fmt.Sprintf("INSERT INTO \"runs\" (\"%s\") %%sVALUES (%s)%%s", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.UseIndexPlaceholders, len(wl), 1, 1))
		} else {
			cache.query = "INSERT INTO \"runs\" %sDEFAULT VALUES%s"
		}

		var queryOutput, queryReturning string

		if len(cache.retMapping) != 0 {
			queryReturning = fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}

		cache.query = fmt.Sprintf(cache.query, queryOutput, queryReturning)
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, cache.query)
		fmt.Fprintln(writer, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRowContext(ctx, cache.query, vals...).Scan(queries.PtrsFromMapping(value, cache.retMapping)...)
	} else {
		_, err = exec.ExecContext(ctx, cache.query, vals...)
	}

	if err != nil {
		return errors.Wrap(err, "models: unable to insert into runs")
	}

	if !cached {
		runInsertCacheMut.Lock()
		runInsertCache[key] = cache
		runInsertCacheMut.Unlock()
	}

	return o.doAfterInsertHooks(ctx, exec)
}

// Update uses an executor to update the Run.
// See boil.Columns.UpdateColumnSet documentation to understand column list inference for updates.
// Update does not automatically update the record in case of default values. Use .Reload() to refresh the records.
func (o *Run) Update(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) (int64, error) {
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		o.UpdatedAt = currTime
	}

	var err error
	if err = o.doBeforeUpdateHooks(ctx, exec); err != nil {
		return 0, err
	}
	key := makeCacheKey(columns, nil)
	runUpdateCacheMut.RLock()
	cache, cached := runUpdateCache[key]
	runUpdateCacheMut.RUnlock()

	if !cached {
		wl := columns.UpdateColumnSet(
			runAllColumns,
			runPrimaryKeyColumns,
		)
		wl = strmangle.SetComplement(wl, runGeneratedColumns)

		if !columns.IsWhitelist() {
			wl = strmangle.SetComplement(wl, []string{"created_at"})
		}
		if len(wl) == 0 {
			return 0, errors.New("models: unable to update runs, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"runs\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, runPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(runType, runMapping, append(wl, runPrimaryKeyColumns...))
		if err != nil {
			return 0, err
		}
	}

	values := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), cache.valueMapping)

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, cache.query)
		fmt.Fprintln(writer, values)
	}
	var result sql.Result
	result, err = exec.ExecContext(ctx, cache.query, values...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update runs row")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by update for runs")
	}

	if !cached {
		runUpdateCacheMut.Lock()
		runUpdateCache[key] = cache
		runUpdateCacheMut.Unlock()
	}

	return rowsAff, o.doAfterUpdateHooks(ctx, exec)
}

// UpdateAll updates all rows with the specified column values.
func (q runQuery) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
	queries.SetUpdate(q.Query, cols)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all for runs")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected for runs")
	}

	return rowsAff, nil
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o RunSlice) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
	ln := int64(len(o))
	if ln == 0 {
		return 0, nil
	}

	if len(cols) == 0 {
		return 0, errors.New("models: update all requires at least one column argument")
	}

	colNames := make([]string, len(cols))
	args := make([]interface{}, len(cols))

	i := 0
	for name, value := range cols {
		colNames[i] = name
		args[i] = value
		i++
	}

	// Append all of the primary key values for each column
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), runPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf("UPDATE \"runs\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), len(colNames)+1, runPrimaryKeyColumns, len(o)))

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args...)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all in run slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected all in update all run")
	}
	return rowsAff, nil
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
// See boil.Columns documentation for how to properly use updateColumns and insertColumns.
func (o *Run) Upsert(ctx context.Context, exec boil.ContextExecutor, updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	if o == nil {
		return errors.New("models: no runs provided for upsert")
	}
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		o.UpdatedAt = currTime
		if o.CreatedAt.IsZero() {
			o.CreatedAt = currTime
		}
	}

	if err := o.doBeforeUpsertHooks(ctx, exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(runColumnsWithDefault, o)

	// Build cache key in-line uglily - mysql vs psql problems
	buf := strmangle.GetBuffer()
	if updateOnConflict {
		buf.WriteByte('t')
	} else {
		buf.WriteByte('f')
	}
	buf.WriteByte('.')
	for _, c := range conflictColumns {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(updateColumns.Kind))
	for _, c := range updateColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(insertColumns.Kind))
	for _, c := range insertColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range nzDefaults {
		buf.WriteString(c)
	}
	key := buf.String()
	strmangle.PutBuffer(buf)

	runUpsertCacheMut.RLock()
	cache, cached := runUpsertCache[key]
	runUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		insert, ret := insertColumns.InsertColumnSet(
			runAllColumns,
			runColumnsWithDefault,
			runColumnsWithoutDefault,
			nzDefaults,
		)

		update := updateColumns.UpdateColumnSet(
			runAllColumns,
			runPrimaryKeyColumns,
		)

		insert = strmangle.SetComplement(insert, runGeneratedColumns)
		update = strmangle.SetComplement(update, runGeneratedColumns)

		if updateOnConflict && len(update) == 0 {
			return errors.New("models: unable to upsert runs, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(runPrimaryKeyColumns))
			copy(conflict, runPrimaryKeyColumns)
		}
		cache.query = buildUpsertQueryPostgres(dialect, "\"runs\"", updateOnConflict, ret, update, conflict, insert)

		cache.valueMapping, err = queries.BindMapping(runType, runMapping, insert)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(runType, runMapping, ret)
			if err != nil {
				return err
			}
		}
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)
	var returns []interface{}
	if len(cache.retMapping) != 0 {
		returns = queries.PtrsFromMapping(value, cache.retMapping)
	}

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, cache.query)
		fmt.Fprintln(writer, vals)
	}
	if len(cache.retMapping) != 0 {
		err = exec.QueryRowContext(ctx, cache.query, vals...).Scan(returns...)
		if errors.Is(err, sql.ErrNoRows) {
			err = nil // Postgres doesn't return anything when there's no update
		}
	} else {
		_, err = exec.ExecContext(ctx, cache.query, vals...)
	}
	if err != nil {
		return errors.Wrap(err, "models: unable to upsert runs")
	}

	if !cached {
		runUpsertCacheMut.Lock()
		runUpsertCache[key] = cache
		runUpsertCacheMut.Unlock()
	}

	return o.doAfterUpsertHooks(ctx, exec)
}

// Delete deletes a single Run record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Run) Delete(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if o == nil {
		return 0, errors.New("models: no Run provided for delete")
	}

	if err := o.doBeforeDeleteHooks(ctx, exec); err != nil {
		return 0, err
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), runPrimaryKeyMapping)
	sql := "DELETE FROM \"runs\" WHERE \"id\"=$1"

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args...)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete from runs")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by delete for runs")
	}

	if err := o.doAfterDeleteHooks(ctx, exec); err != nil {
		return 0, err
	}

	return rowsAff, nil
}

// DeleteAll deletes all matching rows.
func (q runQuery) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if q.Query == nil {
		return 0, errors.New("models: no runQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from runs")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for runs")
	}

	return rowsAff, nil
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o RunSlice) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if len(o) == 0 {
		return 0, nil
	}

	if len(runBeforeDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doBeforeDeleteHooks(ctx, exec); err != nil {
				return 0, err
			}
		}
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), runPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "DELETE FROM \"runs\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, runPrimaryKeyColumns, len(o))

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from run slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for runs")
	}

	if len(runAfterDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterDeleteHooks(ctx, exec); err != nil {
				return 0, err
			}
		}
	}

	return rowsAff, nil
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Run) Reload(ctx context.Context, exec boil.ContextExecutor) error {
	ret, err := FindRun(ctx, exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *RunSlice) ReloadAll(ctx context.Context, exec boil.ContextExecutor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	slice := RunSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), runPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "SELECT \"runs\".* FROM \"runs\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, runPrimaryKeyColumns, len(*o))

	q := queries.Raw(sql, args...)

	err := q.Bind(ctx, exec, &slice)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in RunSlice")
	}

	*o = slice

	return nil
}

// RunExists checks if the Run row exists.
func RunExists(ctx context.Context, exec boil.ContextExecutor, iD int) (bool, error) {
	var exists bool
	sql := "select exists(select 1 from \"runs\" where \"id\"=$1 limit 1)"

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, iD)
	}
	row := exec.QueryRowContext(ctx, sql, iD)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if runs exists")
	}

	return exists, nil
}

// Exists checks if the Run row exists.
func (o *Run) Exists(ctx context.Context, exec boil.ContextExecutor) (bool, error) {
	return RunExists(ctx, exec, o.ID)
}
