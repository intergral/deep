package deepql

import "errors"

type RootExpr struct {
	trigger *trigger
	command *command
	search  *search
}

func (e RootExpr) validate() error {
	var errs []error
	if e.trigger != nil && e.command != nil {
		return errors.New("fatal error: cannot define a trigger and a command")
	}
	if e.trigger != nil {
		err := e.trigger.validate()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if e.command != nil {
		err := e.command.validate()
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (e RootExpr) IsSearch() bool {
	if e.search != nil {
		return true
	}
	return false
}

func (e RootExpr) IsTrigger() bool {
	if e.trigger != nil {
		return true
	}
	return false
}

func (e RootExpr) IsCommand() bool {
	if e.command != nil {
		return true
	}
	return false
}
