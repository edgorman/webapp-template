package entity

// Caller is a verified identity, populated entirely from a provider's signed token payload.
type Caller struct {
	UID   string `json:"uid"`
	Email string `json:"email"`
	Name  string `json:"name"`
	// EmailVerified is the provider's own assertion that this caller controls Email, carried
	// separately because Email alone is not evidence of anything: a provider that lets an account
	// hold an unverified address would otherwise let it hold somebody else's. Nothing keys access
	// on an address, so this is here for whatever treats an address as an identity, and that thing
	// has to check it first.
	EmailVerified bool `json:"emailVerified"`
}
