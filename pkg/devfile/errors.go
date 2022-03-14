package devfile

import "fmt"

//NoDevfileFound returns an error if no devfile was found
type NoDevfileFound struct {
	location string
	err      error
}

func (e *NoDevfileFound) Error() string {
	errMsg := fmt.Sprintf("unable to find devfile in the specified location %s", e.location)
	if e.err != nil {
		errMsg = fmt.Sprintf("%s due to %v", errMsg, e.err)
	}
	return errMsg
}
