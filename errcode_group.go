package qqbotsdk

// Error codes met while calling the group management endpoints.
//
// These are not taken from a documented table: the group management pages
// publish no error code list of their own, so both were observed against the
// live platform and named here for the caller's benefit.
const (
	// ErrGroupNoAPIPermission means the bot application is not allowed to call
	// the endpoint at all.
	//
	// It is an application level gate rather than a per-call refusal, and it is
	// not something the bot can work around by being a group administrator:
	// production showed the member list, the single member call, the blacklist
	// query and change, and the batch removal all answered this while muting a
	// member in the same group succeeded.
	ErrGroupNoAPIPermission OpenAPIErrorCode = 40012010
	// ErrGroupOperationFailed means the platform could not carry the change
	// out, and the documentation's advice is to retry later. Deleting an
	// automatic join approval strategy twice answers this rather than a
	// not-found code, so it cannot be read as "already gone".
	ErrGroupOperationFailed OpenAPIErrorCode = 50105002
)

// groupErrorNames maps the observed group codes to their platform text.
var groupErrorNames = map[OpenAPIErrorCode]string{
	ErrGroupNoAPIPermission: "应用无接口访问权限",
	ErrGroupOperationFailed: "处理失败，请稍后重试",
}
