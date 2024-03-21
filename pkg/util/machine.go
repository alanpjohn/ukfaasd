package util

import "kraftkit.sh/api/machine/v1alpha1"

func IsActive(state v1alpha1.MachineState) bool {
	return (state == v1alpha1.MachineStateCreated ||
		state == v1alpha1.MachineStateRunning)
}
