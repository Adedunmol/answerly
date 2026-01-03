package profiles

import "time"

type UpdateProfileBody struct {
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	DateOfBirth time.Time `json:"date_of_birth"`
	Gender      string    `json:"gender"`
	University  string    `json:"university"`
	Faculty     string    `json:"faculty"`
	Location    string    `json:"location"`
}
