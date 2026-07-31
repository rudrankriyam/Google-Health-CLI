package registry

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type catalogEntry struct {
	name       string
	endpoint   string
	filter     string
	recordType string
	scope      string
	operations string
}

var catalogFixture = []catalogEntry{
	{"Active Energy Burned", "active-energy-burned", "active_energy_burned", "Interval", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Active Minutes", "active-minutes", "active_minutes", "Interval", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Active Zone Minutes", "active-zone-minutes", "active_zone_minutes", "Interval", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Activity Level", "activity-level", "activity_level", "Interval", "activity_and_fitness", "list reconcile"},
	{"Altitude", "altitude", "altitude", "Interval", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Blood Glucose", "blood-glucose", "blood_glucose", "Sample", "health_metrics_and_measurements", "list get reconcile rollup dailyRollUp"},
	{"Body Fat", "body-fat", "body_fat", "Sample", "health_metrics_and_measurements", "list get reconcile rollup dailyRollUp create update batchDelete"},
	{"Calories In Heart Rate Zone", "calories-in-heart-rate-zone", "calories_in_heart_rate_zone", "Interval", "activity_and_fitness", "rollup dailyRollUp"},
	{"Core Body Temperature", "core-body-temperature", "core_body_temperature", "Sample", "health_metrics_and_measurements", "list get reconcile rollup dailyRollUp"},
	{"Daily Heart Rate Variability", "daily-heart-rate-variability", "daily_heart_rate_variability", "Daily", "health_metrics_and_measurements", "list reconcile"},
	{"Daily Heart Rate Zones", "daily-heart-rate-zones", "daily_heart_rate_zones", "Daily", "health_metrics_and_measurements", "list reconcile"},
	{"Daily Oxygen Saturation", "daily-oxygen-saturation", "daily_oxygen_saturation", "Daily", "health_metrics_and_measurements", "list reconcile"},
	{"Daily Respiratory Rate", "daily-respiratory-rate", "daily_respiratory_rate", "Daily", "health_metrics_and_measurements", "list reconcile"},
	{"Daily Resting Heart Rate", "daily-resting-heart-rate", "daily_resting_heart_rate", "Daily", "health_metrics_and_measurements", "list reconcile"},
	{"Daily Sleep Temperature Derivations", "daily-sleep-temperature-derivations", "daily_sleep_temperature_derivations", "Daily", "health_metrics_and_measurements", "list reconcile"},
	{"Daily VO2 Max", "daily-vo2-max", "daily_vo2_max", "Daily", "activity_and_fitness", "list reconcile"},
	{"Distance", "distance", "distance", "Interval", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Electrocardiogram (ECG)", "electrocardiogram", "electrocardiogram", "Session", "ecg", "list"},
	{"Exercise", "exercise", "exercise", "Session", "activity_and_fitness", "list get reconcile create update batchDelete"},
	{"Floors", "floors", "floors", "Interval", "activity_and_fitness", "reconcile rollup dailyRollUp"},
	{"Food", "food", "food", "Food", "nutrition", "list get"},
	{"Food Measurement Unit", "food-measurement-unit", "food_measurement_unit", "Food", "nutrition", "list get"},
	{"Heart Rate", "heart-rate", "heart_rate", "Sample", "health_metrics_and_measurements", "list reconcile rollup dailyRollUp"},
	{"Heart Rate Variability", "heart-rate-variability", "heart_rate_variability", "Sample", "health_metrics_and_measurements", "list reconcile"},
	{"Height", "height", "height", "Sample", "health_metrics_and_measurements", "list get reconcile create update batchDelete"},
	{"Hydration Log", "hydration-log", "hydration_log", "Session", "nutrition", "list get reconcile rollup dailyRollUp create update batchDelete"},
	{"Irregular Rhythm Notification", "irregular-rhythm-notification", "irregular_rhythm_notification", "Session", "irn", "list"},
	{"Nutrition Log", "nutrition-log", "nutrition_log", "Sample", "nutrition", "list get reconcile rollup dailyRollUp create update batchDelete"},
	{"Oxygen Saturation", "oxygen-saturation", "oxygen_saturation", "Sample", "health_metrics_and_measurements", "list reconcile"},
	{"Respiratory Rate Sleep Summary", "respiratory-rate-sleep-summary", "respiratory_rate_sleep_summary", "Sample", "health_metrics_and_measurements", "list reconcile"},
	{"Run VO2 Max", "run-vo2-max", "run_vo2_max", "Sample", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Sedentary Period", "sedentary-period", "sedentary_period", "Interval", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Sleep", "sleep", "sleep", "Session", "sleep", "list get reconcile create update batchDelete"},
	{"Steps", "steps", "steps", "Interval", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Swim Lengths Data", "swim-lengths-data", "swim_lengths_data", "Interval", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Time in Heart Rate Zone", "time-in-heart-rate-zone", "time_in_heart_rate_zone", "Interval", "activity_and_fitness", "list reconcile rollup dailyRollUp"},
	{"Total Calories", "total-calories", "total_calories", "Interval", "activity_and_fitness", "rollup dailyRollUp"},
	{"VO2 Max", "vo2-max", "vo2_max", "Sample", "activity_and_fitness", "list reconcile"},
	{"Weight", "weight", "weight", "Sample", "health_metrics_and_measurements", "list get reconcile rollup dailyRollUp create update batchDelete"},
}

func TestRegistryMatchesDocumentedSurface(t *testing.T) {
	if got, want := len(Types()), 39; got != want {
		t.Fatalf("Types() length = %d, want %d", got, want)
	}
	if got, want := len(RESTOperations()), 25; got != want {
		t.Fatalf("RESTOperations() length = %d, want %d", got, want)
	}
}

func TestRegistryMatchesCompleteCatalogFixture(t *testing.T) {
	actual := make(map[string]DataType, len(Types()))
	for _, dataType := range Types() {
		actual[dataType.EndpointName] = dataType
	}
	if got, want := len(actual), len(catalogFixture); got != want {
		t.Fatalf("unique endpoints = %d, want %d", got, want)
	}

	for _, want := range catalogFixture {
		got, ok := actual[want.endpoint]
		if !ok {
			t.Errorf("missing data type %q", want.endpoint)
			continue
		}
		if got.Name != want.name || got.FilterName != want.filter || got.RecordType != want.recordType || got.Scope != want.scope {
			t.Errorf("%s identity = {%q %q %q %q}, want {%q %q %q %q}", want.endpoint, got.Name, got.FilterName, got.RecordType, got.Scope, want.name, want.filter, want.recordType, want.scope)
		}
		if wantOperations := strings.Fields(want.operations); !reflect.DeepEqual(got.Operations, wantOperations) {
			t.Errorf("%s operations = %v, want %v", want.endpoint, got.Operations, wantOperations)
		}
	}
}

func TestRegistryIdentifiersAndMetadataAreValid(t *testing.T) {
	endpointPattern := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	filterPattern := regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*$`)
	validOperations := map[string]bool{
		"list": true, "get": true, "reconcile": true, "rollup": true,
		"dailyRollUp": true, "create": true, "update": true, "batchDelete": true,
	}
	seenFilters := map[string]string{}
	for _, dataType := range Types() {
		if !endpointPattern.MatchString(dataType.EndpointName) {
			t.Errorf("invalid endpoint identifier %q", dataType.EndpointName)
		}
		if !filterPattern.MatchString(dataType.FilterName) {
			t.Errorf("invalid filter identifier %q", dataType.FilterName)
		}
		if previous, exists := seenFilters[dataType.FilterName]; exists {
			t.Errorf("filter %q is shared by %s and %s", dataType.FilterName, previous, dataType.EndpointName)
		}
		seenFilters[dataType.FilterName] = dataType.EndpointName
		if dataType.Source != dataTypesSource {
			t.Errorf("%s source = %q", dataType.EndpointName, dataType.Source)
		}
		if dataType.SourceURL != dataTypesSourceURL {
			t.Errorf("%s source URL = %q", dataType.EndpointName, dataType.SourceURL)
		}
		if dataType.ReadScope == "" {
			t.Errorf("%s has no read scope", dataType.EndpointName)
		}
		seenOperations := map[string]bool{}
		for _, operation := range dataType.Operations {
			if !validOperations[operation] {
				t.Errorf("%s has invalid operation %q", dataType.EndpointName, operation)
			}
			if seenOperations[operation] {
				t.Errorf("%s repeats operation %q", dataType.EndpointName, operation)
			}
			seenOperations[operation] = true
		}
		if dataType.TimeFilter == nil {
			if dataType.DefaultTimePath != "" || dataType.MaxQueryDays != 0 {
				t.Errorf("%s has time metadata without a time filter", dataType.EndpointName)
			}
		} else {
			if dataType.DefaultTimePath != dataType.TimeFilter.Path {
				t.Errorf("%s default path %q differs from filter path %q", dataType.EndpointName, dataType.DefaultTimePath, dataType.TimeFilter.Path)
			}
			if dataType.MaxQueryDays != 14 && dataType.MaxQueryDays != 90 {
				t.Errorf("%s max query days = %d", dataType.EndpointName, dataType.MaxQueryDays)
			}
		}
	}
}

func TestScopeCatalog(t *testing.T) {
	wantRead := []string{
		ScopeProfileRead, ScopeSettingsRead, ScopeActivityRead, ScopeMetricsRead, ScopeSleepRead,
		ScopeNutritionRead, ScopeECGRead, ScopeIRNRead, ScopeLocationRead,
	}
	wantWrite := []string{
		ScopeProfileWrite, ScopeSettingsWrite, ScopeActivityWrite, ScopeMetricsWrite, ScopeSleepWrite, ScopeNutritionWrite,
	}
	if got := ReadOnlyScopes(); !reflect.DeepEqual(got, wantRead) {
		t.Fatalf("ReadOnlyScopes() = %v, want %v", got, wantRead)
	}
	if got := WriteScopes(); !reflect.DeepEqual(got, wantWrite) {
		t.Fatalf("WriteScopes() = %v, want %v", got, wantWrite)
	}
	for _, scope := range WriteScopes() {
		if !strings.HasSuffix(scope, ".writeonly") {
			t.Errorf("write scope %q does not end in .writeonly", scope)
		}
	}
	wantAll := append(append([]string(nil), wantRead...), wantWrite...)
	if got := AllScopes(); !reflect.DeepEqual(got, wantAll) {
		t.Fatalf("AllScopes() = %v, want %v", got, wantAll)
	}
	if got, want := ManagementScopes(), []string{ScopeCloudPlatform}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ManagementScopes() = %v, want %v", got, want)
	}
	if got, want := SubscriberScopes(), ManagementScopes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("SubscriberScopes() = %v, want %v", got, want)
	}
}

func TestDataTypeScopes(t *testing.T) {
	for _, dataType := range Types() {
		wantRead, wantWrite := scopesFor(dataType.Scope)
		if dataType.ReadScope != wantRead || dataType.WriteScope != wantWrite {
			t.Errorf("%s scopes = (%q, %q), want (%q, %q)", dataType.EndpointName, dataType.ReadScope, dataType.WriteScope, wantRead, wantWrite)
		}
		if (dataType.Scope == "ecg" || dataType.Scope == "irn") && dataType.WriteScope != "" {
			t.Errorf("read-only type %s has write scope %q", dataType.EndpointName, dataType.WriteScope)
		}
	}
}

func TestCatalogFeatureSets(t *testing.T) {
	var webhooks, trueZeros, shortRanges, requiredFilters []string
	for _, dataType := range Types() {
		if dataType.WebhookSupported {
			webhooks = append(webhooks, dataType.EndpointName)
		}
		if dataType.TrueZerosSupported {
			trueZeros = append(trueZeros, dataType.EndpointName)
		}
		if dataType.MaxQueryDays == 14 {
			shortRanges = append(shortRanges, dataType.EndpointName)
		}
		if dataType.FilterRequired {
			requiredFilters = append(requiredFilters, dataType.EndpointName)
		}
	}
	wantWebhooks := []string{
		"active-zone-minutes", "activity-level", "altitude", "blood-glucose", "body-fat",
		"calories-in-heart-rate-zone", "daily-heart-rate-variability", "daily-heart-rate-zones",
		"daily-oxygen-saturation", "daily-respiratory-rate", "daily-resting-heart-rate",
		"daily-sleep-temperature-derivations", "distance", "exercise", "floors", "heart-rate",
		"heart-rate-variability", "height", "hydration-log", "nutrition-log",
		"respiratory-rate-sleep-summary", "run-vo2-max", "sedentary-period", "sleep", "steps",
		"time-in-heart-rate-zone", "weight",
	}
	wantTrueZeros := []string{"altitude", "distance", "floors", "steps", "total-calories"}
	wantShortRanges := []string{"active-minutes", "calories-in-heart-rate-zone", "heart-rate", "total-calories"}
	wantRequiredFilters := []string{"total-calories"}
	for _, pair := range []struct {
		name string
		got  []string
		want []string
	}{
		{"webhooks", webhooks, wantWebhooks},
		{"true zeros", trueZeros, wantTrueZeros},
		{"short ranges", shortRanges, wantShortRanges},
		{"required filters", requiredFilters, wantRequiredFilters},
	} {
		if !reflect.DeepEqual(pair.got, pair.want) {
			t.Errorf("%s = %v, want %v", pair.name, pair.got, pair.want)
		}
	}
}

func TestTimeFilterMetadata(t *testing.T) {
	for _, dataType := range Types() {
		var wantPath string
		var wantLiteral TimeLiteral
		wantUpperBound := true
		switch {
		case dataType.EndpointName == "sleep":
			wantPath, wantLiteral = "sleep.interval.end_time", TimeLiteralRFC3339
		case dataType.EndpointName == "electrocardiogram":
			wantPath, wantLiteral, wantUpperBound = "electrocardiogram.interval.start_time", TimeLiteralRFC3339, false
		case dataType.EndpointName == "nutrition-log":
			wantPath, wantLiteral = "nutrition_log.interval.civil_start_time", TimeLiteralCivil
		case dataType.RecordType == "Interval" || dataType.RecordType == "Session":
			wantPath, wantLiteral = dataType.FilterName+".interval.civil_start_time", TimeLiteralCivil
		case dataType.RecordType == "Sample":
			wantPath, wantLiteral = dataType.FilterName+".sample_time.physical_time", TimeLiteralRFC3339
		case dataType.RecordType == "Daily":
			wantPath, wantLiteral = dataType.FilterName+".date", TimeLiteralDateOnly
		case dataType.RecordType == "Food":
			if dataType.TimeFilter != nil {
				t.Errorf("%s unexpectedly has a time filter", dataType.EndpointName)
			}
			continue
		}
		if dataType.TimeFilter == nil {
			t.Errorf("%s is missing a time filter", dataType.EndpointName)
			continue
		}
		if dataType.TimeFilter.Path != wantPath || dataType.TimeFilter.Literal != wantLiteral || dataType.TimeFilter.SupportsUpperBound != wantUpperBound {
			t.Errorf("%s time filter = %+v, want path=%q literal=%q upper=%t", dataType.EndpointName, dataType.TimeFilter, wantPath, wantLiteral, wantUpperBound)
		}
	}
}

func TestBuildRangeFilter(t *testing.T) {
	tests := []struct {
		name     string
		dataType string
		from     string
		to       string
		want     string
		wantErr  bool
	}{
		{"sample physical range", "heart-rate", "2026-05-08T00:00:00Z", "2026-05-09T00:00:00Z", `heart_rate.sample_time.physical_time >= "2026-05-08T00:00:00Z" AND heart_rate.sample_time.physical_time < "2026-05-09T00:00:00Z"`, false},
		{"interval civil range", "steps", "2026-05-08", "2026-05-09", `steps.interval.civil_start_time >= "2026-05-08" AND steps.interval.civil_start_time < "2026-05-09"`, false},
		{"daily date range", "daily-heart-rate-zones", "2026-05-08", "2026-05-09", `daily_heart_rate_zones.date >= "2026-05-08" AND daily_heart_rate_zones.date < "2026-05-09"`, false},
		{"nutrition session range", "nutrition-log", "2026-05-08", "2026-05-09", `nutrition_log.interval.civil_start_time >= "2026-05-08" AND nutrition_log.interval.civil_start_time < "2026-05-09"`, false},
		{"sleep uses physical end", "sleep", "2026-05-08T00:00:00Z", "2026-05-09T00:00:00Z", `sleep.interval.end_time >= "2026-05-08T00:00:00Z" AND sleep.interval.end_time < "2026-05-09T00:00:00Z"`, false},
		{"ECG lower bound", "electrocardiogram", "2026-05-08T00:00:00Z", "", `electrocardiogram.interval.start_time >= "2026-05-08T00:00:00Z"`, false},
		{"ECG rejects upper bound", "electrocardiogram", "2026-05-08T00:00:00Z", "2026-05-09T00:00:00Z", "", true},
		{"to only", "steps", "", "2026-05-09", `steps.interval.civil_start_time < "2026-05-09"`, false},
		{"food rejects range", "food", "2026-05-08", "", "", true},
		{"sample rejects date", "heart-rate", "2026-05-08", "", "", true},
		{"civil rejects zone", "steps", "2026-05-08T00:00:00Z", "", "", true},
		{"daily rejects time", "daily-heart-rate-zones", "2026-05-08T00:00:00Z", "", "", true},
		{"range must increase", "steps", "2026-05-09", "2026-05-08", "", true},
		{"short range accepts 14 days", "heart-rate", "2026-05-01T00:00:00Z", "2026-05-15T00:00:00Z", `heart_rate.sample_time.physical_time >= "2026-05-01T00:00:00Z" AND heart_rate.sample_time.physical_time < "2026-05-15T00:00:00Z"`, false},
		{"short range rejects over 14 days", "heart-rate", "2026-05-01T00:00:00Z", "2026-05-15T00:00:01Z", "", true},
		{"standard range accepts 90 days", "steps", "2026-01-01", "2026-04-01", `steps.interval.civil_start_time >= "2026-01-01" AND steps.interval.civil_start_time < "2026-04-01"`, false},
		{"standard range rejects over 90 days", "steps", "2026-01-01", "2026-04-02", "", true},
		{"required filter", "total-calories", "", "", "", true},
		{"optional empty filter", "steps", "", "", "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataType, ok := Lookup(test.dataType)
			if !ok {
				t.Fatalf("Lookup(%q) failed", test.dataType)
			}
			got, err := BuildRangeFilter(dataType, test.from, test.to)
			if (err != nil) != test.wantErr {
				t.Fatalf("BuildRangeFilter() error = %v, wantErr %t", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("BuildRangeFilter() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLegacyFilterFromRangeUsesValidatedBuilder(t *testing.T) {
	heartRate, _ := Lookup("heart-rate")
	got := FilterFromRange(heartRate, "2026-05-08T00:00:00Z", "2026-05-09T00:00:00Z")
	want := `heart_rate.sample_time.physical_time >= "2026-05-08T00:00:00Z" AND heart_rate.sample_time.physical_time < "2026-05-09T00:00:00Z"`
	if got != want {
		t.Fatalf("FilterFromRange() = %q, want %q", got, want)
	}
	ecg, _ := Lookup("electrocardiogram")
	if got := FilterFromRange(ecg, "2026-05-08T00:00:00Z", "2026-05-09T00:00:00Z"); got != "" {
		t.Fatalf("invalid ECG range produced %q", got)
	}
}

func TestLookupAcceptsEndpointFilterAndDisplayNames(t *testing.T) {
	tests := []string{"heart-rate-variability", "heart_rate_variability", "Heart Rate Variability"}
	for _, test := range tests {
		dataType, ok := Lookup(test)
		if !ok || dataType.EndpointName != "heart-rate-variability" {
			t.Errorf("Lookup(%q) = (%q, %t)", test, dataType.EndpointName, ok)
		}
	}
}

func TestOperationAliases(t *testing.T) {
	bodyFat, _ := Lookup("body-fat")
	for _, operation := range []string{"update", "patch", "batchDelete", "delete", "dailyRollUp", "dailyRollup"} {
		if !HasOperation(bodyFat, operation) {
			t.Errorf("body-fat should accept operation alias %q", operation)
		}
	}
}

func TestTypesAndLookupReturnDeepCopies(t *testing.T) {
	types := Types()
	types[0].Operations[0] = "corrupted"
	types[0].TimeFilter.Path = "corrupted"
	fresh, ok := Lookup(types[0].EndpointName)
	if !ok {
		t.Fatal("Lookup() failed")
	}
	if fresh.Operations[0] == "corrupted" || fresh.TimeFilter.Path == "corrupted" {
		t.Fatal("Types() exposed mutable registry storage")
	}

	fresh.Operations[0] = "corrupted"
	fresh.TimeFilter.Path = "corrupted"
	again, _ := Lookup(fresh.EndpointName)
	if again.Operations[0] == "corrupted" || again.TimeFilter.Path == "corrupted" {
		t.Fatal("Lookup() exposed mutable registry storage")
	}

	allScopes := AllScopes()
	allScopes[0] = "corrupted"
	if AllScopes()[0] == "corrupted" {
		t.Fatal("AllScopes() exposed mutable registry storage")
	}
}

func TestRESTOperationsMatchPublicReference(t *testing.T) {
	want := map[string]string{
		"projects.subscribers.create":                  "POST|/v4/{parent=projects/*}/subscribers|subscribers",
		"projects.subscribers.delete":                  "DELETE|/v4/{name=projects/*/subscribers/*}|subscribers",
		"projects.subscribers.list":                    "GET|/v4/{parent=projects/*}/subscribers|subscribers",
		"projects.subscribers.patch":                   "PATCH|/v4/{subscriber.name=projects/*/subscribers/*}|subscribers",
		"projects.subscribers.subscriptions.create":    "POST|/v4/{parent=projects/*/subscribers/*}/subscriptions|subscriptions",
		"projects.subscribers.subscriptions.delete":    "DELETE|/v4/{name=projects/*/subscribers/*/subscriptions/*}|subscriptions",
		"projects.subscribers.subscriptions.list":      "GET|/v4/{parent=projects/*/subscribers/*}/subscriptions|subscriptions",
		"projects.subscribers.subscriptions.patch":     "PATCH|/v4/{subscription.name=projects/*/subscribers/*/subscriptions/*}|subscriptions",
		"users.getIdentity":                            "GET|/v4/{name=users/*/identity}|users",
		"users.getIrnProfile":                          "GET|/v4/{name=users/*/irnProfile}|users",
		"users.getProfile":                             "GET|/v4/{name=users/*/profile}|users",
		"users.getSettings":                            "GET|/v4/{name=users/*/settings}|users",
		"users.updateProfile":                          "PATCH|/v4/{profile.name=users/*/profile}|users",
		"users.updateSettings":                         "PATCH|/v4/{settings.name=users/*/settings}|users",
		"users.dataTypes.dataPoints.batchDelete":       "POST|/v4/{parent=users/*/dataTypes/*}/dataPoints:batchDelete|dataPoints",
		"users.dataTypes.dataPoints.create":            "POST|/v4/{parent=users/*/dataTypes/*}/dataPoints|dataPoints",
		"users.dataTypes.dataPoints.dailyRollUp":       "POST|/v4/{parent=users/*/dataTypes/*}/dataPoints:dailyRollUp|dataPoints",
		"users.dataTypes.dataPoints.exportExerciseTcx": "GET|/v4/{name=users/*/dataTypes/*/dataPoints/*}:exportExerciseTcx|dataPoints",
		"users.dataTypes.dataPoints.get":               "GET|/v4/{name=users/*/dataTypes/*/dataPoints/*}|dataPoints",
		"users.dataTypes.dataPoints.list":              "GET|/v4/{parent=users/*/dataTypes/*}/dataPoints|dataPoints",
		"users.dataTypes.dataPoints.patch":             "PATCH|/v4/{dataPoint.name=users/*/dataTypes/*/dataPoints/*}|dataPoints",
		"users.dataTypes.dataPoints.reconcile":         "GET|/v4/{parent=users/*/dataTypes/*}/dataPoints:reconcile|dataPoints",
		"users.dataTypes.dataPoints.rollUp":            "POST|/v4/{parent=users/*/dataTypes/*}/dataPoints:rollUp|dataPoints",
		"users.pairedDevices.get":                      "GET|/v4/{name=users/*/pairedDevices/*}|pairedDevices",
		"users.pairedDevices.list":                     "GET|/v4/{parent=users/*}/pairedDevices|pairedDevices",
	}
	got := map[string]string{}
	for _, operation := range RESTOperations() {
		if _, duplicate := got[operation.Name]; duplicate {
			t.Errorf("duplicate REST operation %q", operation.Name)
		}
		got[operation.Name] = strings.Join([]string{operation.Method, operation.Path, operation.Category}, "|")
	}
	if !reflect.DeepEqual(got, want) {
		gotKeys, wantKeys := make([]string, 0, len(got)), make([]string, 0, len(want))
		for key := range got {
			gotKeys = append(gotKeys, key)
		}
		for key := range want {
			wantKeys = append(wantKeys, key)
		}
		sort.Strings(gotKeys)
		sort.Strings(wantKeys)
		t.Fatalf("REST operations differ\ngot keys:  %v\nwant keys: %v", gotKeys, wantKeys)
	}
}
