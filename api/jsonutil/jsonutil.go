package jsonutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"net/http"
	"reflect"
	"strings"
)

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func WriteJSONResponse(responseWriter http.ResponseWriter, data interface{}, statusCode int) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)

	if err := json.NewEncoder(responseWriter).Encode(data); err != nil {
		http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
		return
	}
}

func UnmarshalJsonResponse[T any](request *http.Request) (T, error) {
	var body T

	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		return body, fmt.Errorf("error decoding body")
	}

	validationErr := Validate(body)
	if validationErr != nil {
		return body, validationErr
	}

	return body, nil
}

var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return "" // skip if json:"-"
		}
		return name
	})
}

func Validate(i any) error {
	err := validate.Struct(i)
	if err == nil {
		return nil
	}

	var invalidValidationError *validator.InvalidValidationError
	if errors.As(err, &invalidValidationError) {
		fmt.Println(err)
		return invalidValidationError
	}

	var validationErrors []string
	for _, fe := range err.(validator.ValidationErrors) {

		message := getErrorMessage(fe)
		validationErrors = append(validationErrors, fmt.Sprintf("%s: %s", fe.Field(), message))
	}

	return fmt.Errorf("validation failed: %s", strings.Join(validationErrors, ", "))
}

func getErrorMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", e.Field())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", e.Field(), e.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", e.Field(), e.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of %s", e.Field(), e.Param())
	default:
		return fmt.Sprintf("%s is invalid", e.Field())
	}
}
