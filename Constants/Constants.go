package constants

const (
	KubeSystemNamespace = "kube-system"
	HelmChart           = "ostore-1.5.0"

	// ANSI Color Codes
	Reset          = "\x1b[0m"
	Bold           = "\x1b[1m"
	FgGreen        = "\x1b[32m"
	FgYellow       = "\x1b[33m"
	FgRed          = "\x1b[31m"
	BoldGreen      = Bold + FgGreen
	Newline        = "\n"
	TwoNewLines    = "\n\n"
	Differentiator = "=========================================================================="
	BoldRed        = Bold + FgRed
)
