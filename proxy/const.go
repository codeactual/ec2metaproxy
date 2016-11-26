package proxy

const (
	// MetadataURL is used if the JSON config file does not override it.
	MetadataURL = "http://169.254.169.254"
	// RoleLabelKey identifies the docker metadata string that holds a role alias.
	// The alias corresponds to the alias-to-ARN mapping in the JSON config file.
	RoleLabelKey = "ec2metaproxy.RoleAlias"
	// PolicyLabelKey identifies the docker metadata string that holds a JSON IAM
	// policy used in the AssumeRole operation.
	PolicyLabelKey = "ec2metaproxy.Policy"
)
