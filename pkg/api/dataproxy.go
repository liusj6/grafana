package api

import "github.com/grafana/grafana/pkg/models"

// swagger:route GET /datasources/id/{name} datasources getDatasourceIdByName
//
// Get data source Id by Name.
//
// If you are running Grafana Enterprise and have Fine-grained access control enabled
// you need to have a permission with action: `datasources:read` and scopes: `datasources:*`, `datasources:name:*` and `datasources:name:test_datasource` (single data source).
//
// Responses:
// 200: getDatasourceIDresponse
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError

// swagger:route POST /datasources/proxy/{id}/{datasource_proxy_route} datasources datasourceProxyPOSTcalls
//
// Data source proxy POST calls.
//
// Proxies all calls to the actual data source. The data source should support POST methods for the specific path and role as defined
//
// Please refer to [updated API](#/datasources/datasourceProxyPOSTByUIDcalls) instead
//
// Deprecated: true
//
// Responses:
// 201:
// 202:
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError

// swagger:route DELETE /datasources/proxy/{id}/{datasource_proxy_route} datasources datasourceProxyDELETEcalls
//
// Data source proxy DELETE calls.
//
// Proxies all calls to the actual data source.
//
// Please refer to [updated API](#/datasources/datasourceProxyDELETEByUIDcalls) instead
//
// Deprecated: true
//
// Responses:
// 202:
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError
func (hs *HTTPServer) ProxyDataSourceRequest(c *models.ReqContext) {
	hs.DataProxy.ProxyDataSourceRequest(c)
}

// swagger:route GET /datasources/proxy/uid/{uid}/{datasource_proxy_route} datasources datasourceProxyGETByUIDcalls
//
// Data source proxy GET calls.
//
// Proxies all calls to the actual data source.
//
// Responses:
// 200:
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError

// swagger:route POST /datasources/proxy/uid/{uid}/{datasource_proxy_route} datasources datasourceProxyPOSTByUIDcalls
//
// Data source proxy POST calls.
//
// Proxies all calls to the actual data source. The data source should support POST methods for the specific path and role as defined
//
// Responses:
// 201:
// 202:
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError

// swagger:route DELETE /datasources/proxy/uid/{uid}/{datasource_proxy_route} datasources datasourceProxyDELETEByUIDcalls
//
// Data source proxy DELETE calls.
//
// Proxies all calls to the actual data source.
//
// Responses:
// 202:
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError
func (hs *HTTPServer) ProxyDataSourceRequestWithUID(c *models.ReqContext) {
	hs.DataProxy.ProxyDatasourceRequestWithUID(c, "")
}

// swagger:parameters datasourceProxyDELETEcalls
type DatasourceProxyDELETEcallsParams struct {
	// in:path
	// required:true
	DatasourceID string `json:"id"`
}

// swagger:parameters datasourceProxyDELETEByUIDcalls
type DatasourceProxyDELETEByUIDcallsParams struct {
	// in:path
	// required:true
	DatasourceUID string `json:"uid"`
}

// swagger:parameters datasourceProxyGETcalls
type DatasourceProxyGETcallsParams struct {
	// in:path
	// required:true
	DatasourceProxyRoute string `json:"datasource_proxy_route"`
	// in:path
	// required:true
	DatasourceID string `json:"id"`
}

// swagger:parameters datasourceProxyGETByUIDcalls
type DatasourceProxyGETByUIDcallsParams struct {
	// in:path
	// required:true
	DatasourceProxyRoute string `json:"datasource_proxy_route"`
	// in:path
	// required:true
	DatasourceUID string `json:"uid"`
}

// swagger:parameters datasourceProxyDELETEcalls
// swagger:parameters datasourceProxyDELETEByUIDcalls
// swagger:parameters callDatasourceResourceWithUID callDatasourceResourceByID
type DatasourceProxyRouteParam struct {
	// in:path
	// required:true
	DatasourceProxyRoute string `json:"datasource_proxy_route"`
}

// swagger:parameters datasourceProxyPOSTcalls
type DatasourceProxyPOSTcallsParams struct {
	// in:body
	// required:true
	DatasourceProxyParam interface{}
	// in:path
	// required:true
	DatasourceProxyRoute string `json:"datasource_proxy_route"`
	// in:path
	// required:true
	DatasourceID string `json:"id"`
}

// swagger:parameters datasourceProxyPOSTByUIDcalls
type DatasourceProxyPOSTByUIDcallsParams struct {
	// in:body
	// required:true
	DatasourceProxyParam interface{}
	// in:path
	// required:true
	DatasourceProxyRoute string `json:"datasource_proxy_route"`
	// in:path
	// required:true
	DatasourceUID string `json:"uid"`
}
