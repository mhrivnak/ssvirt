In pkg/controllers/vdc/controller.go around lines 65 to 91, the startPeriodicReconciliation method runs an infinite loop without a way to stop it, which prevents graceful shutdown and can cause resource leaks. Modify the method to accept a context parameter and use it to listen for cancellation signals, exiting the loop and stopping the ticker when the context is done. Also, update the call to startPeriodicReconciliation in SetupWithManager to pass the manager's context so the reconciliation can be properly stopped during shutdown.

In pkg/database/repositories/vdc.go around lines 86 to 101, the GetByIDString
method returns nil when a record is not found. State that behavior in the
function's doc string.

In pkg/database/repositories/vdc.go around lines 104 to 114, the GetByNamespace
method returns nil for both the VDC and error when the record is not found.
State that behavior in the function's doc string.
