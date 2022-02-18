package tls

type Commands map[string]bool
type UnitRoles map[string]Commands
type OrgRoles map[string]UnitRoles

// GetCommands negotiates the available commands by combining the certificate
// organizations, units and the loaded OrgRoles.
//
// NOTE: This method is not secure because it does not differentiate between
// units and organizations. For example, someone in multiple organizations,
// like, "it" and "hr" can be in sub-units like "admin" and "user". But if the
// "admin" is really in reference to the "hr" organization, this system is not
// smart enough to know that. It would be best to use an org list, and each unit
// has roles pre-pendend with the org name. Ex. org: "hr", unit: "hr.admin"
// would allow for proper fine-grained control.
func GetCommands(config OrgRoles, orgs, units []string) Commands {
	available := Commands{}

	for _, org := range orgs {
		if len(units) == 0 {
			continue
		}

		for _, unit := range units {
			for command, allowed := range config[org][unit] {
				if allowed {
					available[command] = true
				}
			}
		}
	}

	return available
}
