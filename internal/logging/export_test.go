package logging

// SetupStartupLogger exposes setupStartupLogger to the external test package,
// so the startup logger can be checked without writing a startup message.
var SetupStartupLogger = setupStartupLogger

// LogNotifierInfo exposes logNotifierInfo to the external test package, so the
// notifier startup line can be checked on its own.
var LogNotifierInfo = logNotifierInfo

// LogScheduleInfo exposes logScheduleInfo to the external test package, so the
// schedule startup lines can be checked on their own.
var LogScheduleInfo = logScheduleInfo
