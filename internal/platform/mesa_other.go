//go:build !windows

package platform

// InitMesaIfRequired is a no-op on non-Windows platforms
func InitMesaIfRequired() {
}
