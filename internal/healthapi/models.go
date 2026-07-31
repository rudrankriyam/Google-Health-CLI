package healthapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// DataPoint and the rollup value types are intentionally raw JSON. Google
// Health models these resources as growing unions keyed by data type. Keeping
// the union payload raw lets the CLI pass through newly added data types
// without silently discarding fields.
type DataPoint = json.RawMessage
type ReconciledDataPoint = json.RawMessage
type DailyRollupDataPoint = json.RawMessage
type RollupDataPoint = json.RawMessage

type Identity struct {
	Name         string `json:"name,omitempty"`
	HealthUserID string `json:"healthUserId,omitempty"`
	LegacyUserID string `json:"legacyUserId,omitempty"`
}

type IRNProfile struct {
	Name             string `json:"name,omitempty"`
	OnboardingStatus bool   `json:"onboardingStatus"`
	EnrollmentStatus bool   `json:"enrollmentStatus"`
	UpdateTime       string `json:"updateTime,omitempty"`
}

type Date struct {
	Year  int32 `json:"year,omitempty"`
	Month int32 `json:"month,omitempty"`
	Day   int32 `json:"day,omitempty"`
}

type TimeOfDay struct {
	Hours   int32 `json:"hours,omitempty"`
	Minutes int32 `json:"minutes,omitempty"`
	Seconds int32 `json:"seconds,omitempty"`
	Nanos   int32 `json:"nanos,omitempty"`
}

type CivilDateTime struct {
	Date *Date      `json:"date,omitempty"`
	Time *TimeOfDay `json:"time,omitempty"`
}

type CivilTimeInterval struct {
	Start *CivilDateTime `json:"start,omitempty"`
	End   *CivilDateTime `json:"end,omitempty"`
}

type Interval struct {
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
}

type Profile struct {
	Name                                string `json:"name,omitempty"`
	Age                                 *int32 `json:"age,omitempty"`
	MembershipStartDate                 *Date  `json:"membershipStartDate,omitempty"`
	AutoWalkingStrideLengthMM           *int32 `json:"autoWalkingStrideLengthMm,omitempty"`
	AutoRunningStrideLengthMM           *int32 `json:"autoRunningStrideLengthMm,omitempty"`
	UserConfiguredWalkingStrideLengthMM *int32 `json:"userConfiguredWalkingStrideLengthMm,omitempty"`
	UserConfiguredRunningStrideLengthMM *int32 `json:"userConfiguredRunningStrideLengthMm,omitempty"`
}

type UpdateProfileRequest struct {
	Name                                string `json:"name,omitempty"`
	Age                                 *int32 `json:"age,omitempty"`
	UserConfiguredWalkingStrideLengthMM *int32 `json:"userConfiguredWalkingStrideLengthMm,omitempty"`
	UserConfiguredRunningStrideLengthMM *int32 `json:"userConfiguredRunningStrideLengthMm,omitempty"`
}

type TemperatureUnit string
type WeightUnit string
type GlucoseUnit string
type DistanceUnit string
type StrideLengthType string
type SwimUnit string
type HeightUnit string
type WaterUnit string

const (
	TemperatureUnitUnspecified TemperatureUnit = "TEMPERATURE_UNIT_UNSPECIFIED"
	TemperatureUnitCelsius     TemperatureUnit = "TEMPERATURE_UNIT_CELSIUS"
	TemperatureUnitFahrenheit  TemperatureUnit = "TEMPERATURE_UNIT_FAHRENHEIT"

	WeightUnitUnspecified WeightUnit = "WEIGHT_UNIT_UNSPECIFIED"
	WeightUnitPounds      WeightUnit = "WEIGHT_UNIT_POUNDS"
	WeightUnitStone       WeightUnit = "WEIGHT_UNIT_STONE"
	WeightUnitKilograms   WeightUnit = "WEIGHT_UNIT_KILOGRAMS"

	GlucoseUnitUnspecified GlucoseUnit = "GLUCOSE_UNIT_UNSPECIFIED"
	GlucoseUnitMGDL        GlucoseUnit = "GLUCOSE_UNIT_MG_DL"
	GlucoseUnitMMOLL       GlucoseUnit = "GLUCOSE_UNIT_MMOL_L"

	DistanceUnitUnspecified DistanceUnit = "DISTANCE_UNIT_UNSPECIFIED"
	DistanceUnitMiles       DistanceUnit = "DISTANCE_UNIT_MILES"
	DistanceUnitKilometers  DistanceUnit = "DISTANCE_UNIT_KILOMETERS"

	StrideLengthTypeUnspecified StrideLengthType = "STRIDE_LENGTH_TYPE_UNSPECIFIED"
	StrideLengthTypeDefault     StrideLengthType = "STRIDE_LENGTH_TYPE_DEFAULT"
	StrideLengthTypeManual      StrideLengthType = "STRIDE_LENGTH_TYPE_MANUAL"
	StrideLengthTypeAuto        StrideLengthType = "STRIDE_LENGTH_TYPE_AUTO"

	SwimUnitUnspecified SwimUnit = "SWIM_UNIT_UNSPECIFIED"
	SwimUnitMeters      SwimUnit = "SWIM_UNIT_METERS"
	SwimUnitYards       SwimUnit = "SWIM_UNIT_YARDS"

	HeightUnitUnspecified HeightUnit = "HEIGHT_UNIT_UNSPECIFIED"
	HeightUnitInches      HeightUnit = "HEIGHT_UNIT_INCHES"
	HeightUnitCentimeters HeightUnit = "HEIGHT_UNIT_CENTIMETERS"

	WaterUnitUnspecified WaterUnit = "WATER_UNIT_UNSPECIFIED"
	WaterUnitML          WaterUnit = "WATER_UNIT_ML"
	WaterUnitFluidOunces WaterUnit = "WATER_UNIT_FL_OZ"
	WaterUnitCup         WaterUnit = "WATER_UNIT_CUP"
)

type Settings struct {
	Name                    string           `json:"name,omitempty"`
	TemperatureUnit         TemperatureUnit  `json:"temperatureUnit,omitempty"`
	WeightUnit              WeightUnit       `json:"weightUnit,omitempty"`
	GlucoseUnit             GlucoseUnit      `json:"glucoseUnit,omitempty"`
	DistanceUnit            DistanceUnit     `json:"distanceUnit,omitempty"`
	StrideLengthWalkingType StrideLengthType `json:"strideLengthWalkingType,omitempty"`
	StrideLengthRunningType StrideLengthType `json:"strideLengthRunningType,omitempty"`
	SwimUnit                SwimUnit         `json:"swimUnit,omitempty"`
	HeightUnit              HeightUnit       `json:"heightUnit,omitempty"`
	WaterUnit               WaterUnit        `json:"waterUnit,omitempty"`
	TimeZone                string           `json:"timeZone,omitempty"`
	UTCOffset               string           `json:"utcOffset,omitempty"`
	AutoStrideEnabled       *bool            `json:"autoStrideEnabled,omitempty"`
	FoodLanguageCode        string           `json:"foodLanguageCode,omitempty"`
	LanguageLocale          string           `json:"languageLocale,omitempty"`
}

type UpdateSettingsRequest struct {
	Name                    string           `json:"name,omitempty"`
	TemperatureUnit         TemperatureUnit  `json:"temperatureUnit,omitempty"`
	WeightUnit              WeightUnit       `json:"weightUnit,omitempty"`
	GlucoseUnit             GlucoseUnit      `json:"glucoseUnit,omitempty"`
	DistanceUnit            DistanceUnit     `json:"distanceUnit,omitempty"`
	StrideLengthWalkingType StrideLengthType `json:"strideLengthWalkingType,omitempty"`
	StrideLengthRunningType StrideLengthType `json:"strideLengthRunningType,omitempty"`
	SwimUnit                SwimUnit         `json:"swimUnit,omitempty"`
	HeightUnit              HeightUnit       `json:"heightUnit,omitempty"`
	WaterUnit               WaterUnit        `json:"waterUnit,omitempty"`
	TimeZone                string           `json:"timeZone,omitempty"`
	UTCOffset               string           `json:"utcOffset,omitempty"`
	AutoStrideEnabled       *bool            `json:"autoStrideEnabled,omitempty"`
}

type ListDataPointsResponse struct {
	DataPoints    []DataPoint `json:"dataPoints,omitempty"`
	NextPageToken string      `json:"nextPageToken,omitempty"`
}

type ReconcileDataPointsResponse struct {
	DataPoints    []ReconciledDataPoint `json:"dataPoints,omitempty"`
	NextPageToken string                `json:"nextPageToken,omitempty"`
}

type BatchDeleteDataPointsRequest struct {
	Names []string `json:"names"`
}

type DailyRollUpDataPointsRequest struct {
	Range            *CivilTimeInterval `json:"range,omitempty"`
	WindowSizeDays   int32              `json:"windowSizeDays,omitempty"`
	PageSize         int32              `json:"pageSize,omitempty"`
	PageToken        string             `json:"pageToken,omitempty"`
	DataSourceFamily string             `json:"dataSourceFamily,omitempty"`
}

type DailyRollUpDataPointsResponse struct {
	RollupDataPoints []DailyRollupDataPoint `json:"rollupDataPoints,omitempty"`
}

type RollUpDataPointsRequest struct {
	Range            *Interval `json:"range,omitempty"`
	WindowSize       string    `json:"windowSize,omitempty"`
	PageSize         int32     `json:"pageSize,omitempty"`
	PageToken        string    `json:"pageToken,omitempty"`
	DataSourceFamily string    `json:"dataSourceFamily,omitempty"`
}

type RollUpDataPointsResponse struct {
	RollupDataPoints []RollupDataPoint `json:"rollupDataPoints,omitempty"`
	NextPageToken    string            `json:"nextPageToken,omitempty"`
}

type SubscriptionCreatePolicy string

const (
	SubscriptionCreatePolicyUnspecified SubscriptionCreatePolicy = "SUBSCRIPTION_CREATE_POLICY_UNSPECIFIED"
	SubscriptionCreatePolicyAutomatic   SubscriptionCreatePolicy = "AUTOMATIC"
	SubscriptionCreatePolicyManual      SubscriptionCreatePolicy = "MANUAL"
)

type SubscriberState string

const (
	SubscriberStateUnspecified SubscriberState = "STATE_UNSPECIFIED"
	SubscriberStateUnverified  SubscriberState = "UNVERIFIED"
	SubscriberStateActive      SubscriberState = "ACTIVE"
	SubscriberStateInactive    SubscriberState = "INACTIVE"
)

type SubscriberConfig struct {
	DataTypes                []string                 `json:"dataTypes"`
	SubscriptionCreatePolicy SubscriptionCreatePolicy `json:"subscriptionCreatePolicy"`
}

type EndpointAuthorizationInput struct {
	Secret string `json:"secret"`
}

type EndpointAuthorizationStatus struct {
	SecretSet *bool `json:"secretSet,omitempty"`
}

type CreateSubscriberRequest struct {
	EndpointURI           string                     `json:"endpointUri"`
	EndpointAuthorization EndpointAuthorizationInput `json:"endpointAuthorization"`
	SubscriberConfigs     []SubscriberConfig         `json:"subscriberConfigs,omitempty"`
}

type UpdateSubscriberRequest struct {
	Name                  string                      `json:"name,omitempty"`
	EndpointURI           string                      `json:"endpointUri,omitempty"`
	EndpointAuthorization *EndpointAuthorizationInput `json:"endpointAuthorization,omitempty"`
	SubscriberConfigs     []SubscriberConfig          `json:"subscriberConfigs,omitempty"`
}

type Subscriber struct {
	Name                  string                       `json:"name,omitempty"`
	EndpointURI           string                       `json:"endpointUri,omitempty"`
	EndpointAuthorization *EndpointAuthorizationStatus `json:"endpointAuthorization,omitempty"`
	SubscriberConfigs     []SubscriberConfig           `json:"subscriberConfigs,omitempty"`
	State                 SubscriberState              `json:"state,omitempty"`
	CreateTime            string                       `json:"createTime,omitempty"`
	UpdateTime            string                       `json:"updateTime,omitempty"`
}

type ListSubscribersResponse struct {
	Subscribers   []Subscriber `json:"subscribers,omitempty"`
	NextPageToken string       `json:"nextPageToken,omitempty"`
	TotalSize     int32        `json:"totalSize,omitempty"`
}

type CreateSubscriptionRequest struct {
	User      string   `json:"user"`
	DataTypes []string `json:"dataTypes,omitempty"`
}

type UpdateSubscriptionRequest struct {
	Name      string   `json:"name,omitempty"`
	DataTypes []string `json:"dataTypes,omitempty"`
}

type Subscription struct {
	Name      string   `json:"name,omitempty"`
	User      string   `json:"user,omitempty"`
	DataTypes []string `json:"dataTypes,omitempty"`
}

type ListSubscriptionsResponse struct {
	Subscriptions []Subscription `json:"subscriptions,omitempty"`
	NextPageToken string         `json:"nextPageToken,omitempty"`
}

type DeviceType string

const (
	DeviceTypeUnspecified DeviceType = "DEVICE_TYPE_UNSPECIFIED"
	DeviceTypeTracker     DeviceType = "TRACKER"
	DeviceTypeScale       DeviceType = "SCALE"
)

type PairedDevice struct {
	Name          string     `json:"name,omitempty"`
	DeviceType    DeviceType `json:"deviceType,omitempty"`
	BatteryStatus string     `json:"batteryStatus,omitempty"`
	BatteryLevel  *int32     `json:"batteryLevel,omitempty"`
	LastSyncTime  string     `json:"lastSyncTime,omitempty"`
	DeviceVersion string     `json:"deviceVersion,omitempty"`
	MACAddress    string     `json:"macAddress,omitempty"`
	Features      []string   `json:"features,omitempty"`
}

type ListPairedDevicesResponse struct {
	PairedDevices []PairedDevice `json:"pairedDevices,omitempty"`
	NextPageToken string         `json:"nextPageToken,omitempty"`
}

type Empty struct{}

type GoogleStatus struct {
	Code    int               `json:"code,omitempty"`
	Message string            `json:"message,omitempty"`
	Status  string            `json:"status,omitempty"`
	Details []json.RawMessage `json:"details,omitempty"`
}

type Operation struct {
	Name     string          `json:"name,omitempty"`
	Done     *bool           `json:"done,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    *GoogleStatus   `json:"error,omitempty"`
}

func (o Operation) IsDone() bool {
	return o.Done != nil && *o.Done
}

func (o Operation) Err() error {
	if o.Error == nil {
		return nil
	}
	return &OperationError{Name: o.Name, Status: *o.Error}
}

func DecodeOperationResponse[T any](operation Operation) (T, error) {
	var value T
	if err := operation.Err(); err != nil {
		return value, err
	}
	operationName := sanitizeMessage(operation.Name)
	if len(operation.Response) == 0 || string(operation.Response) == "null" {
		return value, fmt.Errorf("operation %q has no response", operationName)
	}
	if err := json.Unmarshal(operation.Response, &value); err != nil {
		return value, fmt.Errorf("decode operation %q response: %w", operationName, err)
	}
	return value, nil
}

type OperationError struct {
	Name   string       `json:"operation,omitempty"`
	Status GoogleStatus `json:"error"`
}

func (e *OperationError) Error() string {
	if e == nil {
		return ""
	}
	message := sanitizeMessage(e.Status.Message)
	if message == "" {
		message = "operation failed"
	}
	name := sanitizeMessage(e.Name)
	if name == "" {
		return message
	}
	return fmt.Sprintf("Google Health operation %s failed: %s", name, message)
}

type APIError struct {
	StatusCode int               `json:"statusCode"`
	Status     string            `json:"status"`
	Code       int               `json:"code,omitempty"`
	APIStatus  string            `json:"apiStatus,omitempty"`
	Message    string            `json:"message,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Domain     string            `json:"domain,omitempty"`
	Details    []json.RawMessage `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	message := sanitizeMessage(e.Message)
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	status := sanitizeMessage(e.Status)
	if status == "" {
		status = "an error"
	}
	if message == "" {
		return fmt.Sprintf("Google Health API returned %s", status)
	}
	return fmt.Sprintf("Google Health API returned %s: %s", status, message)
}

func (e *APIError) Retryable() bool {
	if e == nil {
		return false
	}
	if e.StatusCode == http.StatusRequestTimeout || e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500 {
		return true
	}
	switch e.APIStatus {
	case "DEADLINE_EXCEEDED", "RESOURCE_EXHAUSTED", "UNAVAILABLE":
		return true
	default:
		return false
	}
}

func sanitizeMessage(message string) string {
	message = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, strings.TrimSpace(message))
	message = strings.Join(strings.Fields(message), " ")
	const maximumRunes = 4096
	runes := []rune(message)
	if len(runes) > maximumRunes {
		return string(runes[:maximumRunes]) + "..."
	}
	return message
}
