package failurestrict

import "resource"

func checked(r *resource.Resource) error {
	if err := r.Start(); err != nil { // want `\[start\] Start requires Stop on r before function exit`
		return err
	}
	defer r.Stop()
	return nil
}

func stoppedOnFailure(r *resource.Resource) error {
	if err := r.Start(); err != nil {
		r.Stop()
		return err
	}
	defer r.Stop()
	return nil
}
