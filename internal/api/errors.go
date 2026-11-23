package api

type Error struct {
	Code string `json:"code"`
}

func (e *Error) Error() string { return e.Code }
