package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient represents an HTTP client with authentication
type HTTPClient struct {
	client   *http.Client
	config   *Config
	username string
	password string
}

// NewHTTPClient creates a new HTTP client with the given configuration
func NewHTTPClient(config *Config) *HTTPClient {
	// Create HTTP client with TLS skip verification (for testing)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	
	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}
	
	return &HTTPClient{
		client:   client,
		config:   config,
		username: config.Server.Username,
		password: config.Server.Password,
	}
}

// SetTenantCredentials sets the tenant-specific credentials
func (h *HTTPClient) SetTenantCredentials(tenantIndex int) {
	h.username = h.config.GetTenantUsername(tenantIndex)
	h.password = h.config.Server.Password
}

// getBasicAuthHeader returns the basic authentication header value
func (h *HTTPClient) getBasicAuthHeader() string {
	credentials := fmt.Sprintf("%s:%s", h.username, h.password)
	encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
	return "Basic " + encoded
}

// CreateRole creates a role using SOAP API
func (h *HTTPClient) CreateRole(tenantIndex int) error {
	h.SetTenantCredentials(tenantIndex)
	
	soapBody := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ser="http://service.ws.um.carbon.wso2.org" xmlns:xsd="http://dao.service.ws.um.carbon.wso2.org/xsd">
   <soapenv:Header/>
   <soapenv:Body>
      <ser:addRole>
         <ser:roleName>%s</ser:roleName>       
           <ser:permissions>
            <xsd:action>ui.execute</xsd:action>
            <xsd:resourceId>/permission/admin/login</xsd:resourceId>
         </ser:permissions>
          <ser:permissions>
            <xsd:action>ui.execute</xsd:action>
            <xsd:resourceId>/permission/admin/configure/</xsd:resourceId>
         </ser:permissions>
           <ser:permissions>
            <xsd:action>ui.execute</xsd:action>
             <xsd:resourceId>/permission/admin/manage/</xsd:resourceId>
         </ser:permissions>        
      </ser:addRole> 

   </soapenv:Body>
</soapenv:Envelope>`, h.config.Test.RoleName)

	url := fmt.Sprintf("%s/services/RemoteUserStoreManagerService", h.config.GetServerURL())
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(soapBody)))
	if err != nil {
		return fmt.Errorf("failed to create role request: %v", err)
	}
	
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("SOAPAction", "urn:addRole")
	req.Header.Set("Authorization", h.getBasicAuthHeader())
	
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute role creation request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("role creation failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	fmt.Printf("Role '%s' created successfully for tenant %d\n", h.config.Test.RoleName, tenantIndex)
	
	// Add delay as in JMX (5000ms)
	time.Sleep(5 * time.Second)
	
	return nil
}

// SCIMUser represents a SCIM user payload
type SCIMUser struct {
	Schemas      []string    `json:"schemas"`
	UserName     string      `json:"userName"`
	Password     string      `json:"password"`
	Name         SCIMName    `json:"name"`
	Wso2Extension SCIMWso2Ext `json:"wso2Extension"`
	Emails       []SCIMEmail `json:"emails"`
	Roles        []SCIMRole  `json:"roles"`
}

// SCIMName represents the name part of SCIM user
type SCIMName struct {
	FamilyName string `json:"familyName"`
	GivenName  string `json:"givenName"`
}

// SCIMWso2Ext represents WSO2 extension for SCIM user
type SCIMWso2Ext struct {
	AccountLocked string `json:"accountLocked"`
}

// SCIMEmail represents email in SCIM user
type SCIMEmail struct {
	Primary bool   `json:"primary,omitempty"`
	Value   string `json:"value"`
	Type    string `json:"type"`
}

// SCIMRole represents role in SCIM user
type SCIMRole struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SCIMUserResponse represents the response from SCIM user creation
type SCIMUserResponse struct {
	ID       string `json:"id"`
	UserName string `json:"userName"`
}

func (h *HTTPClient) CreateUser(tenantIndex, userIndex int) (*SCIMUserResponse, error) {
	username := h.config.GetTestUsername(userIndex)
	return h.CreateUserWithName(tenantIndex, username)
}
// CreateUser creates a user using SCIM2 API
func (h *HTTPClient) CreateUserWithName(tenantIndex int, username string) (*SCIMUserResponse, error) {
	h.SetTenantCredentials(tenantIndex)		
	user := SCIMUser{
		Schemas:  []string{},
		UserName: username,
		Password: h.config.Test.UserPassword,
		Name: SCIMName{
			FamilyName: h.config.Test.UsernamePrefix + "Family",
			GivenName:  h.config.Test.UsernamePrefix + "givenName",
		},
		Wso2Extension: SCIMWso2Ext{
			AccountLocked: "false",
		},
		Emails: []SCIMEmail{
			{
				Primary: true,
				Value:   "mail_home.com",
				Type:    "home",
			},
			{
				Value: "mail_work.com",
				Type:  "work",
			},
		},
		Roles: []SCIMRole{
			{
				Type:  "default",
				Value: h.config.Test.RoleName,
			},
		},
	}
	
	userJSON, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user JSON: %v", err)
	}
	
	url := fmt.Sprintf("%s/wso2/scim/Users", h.config.GetServerURL())
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(userJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create user request: %v", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", h.getBasicAuthHeader())
	
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute user creation request: %v", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user creation failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var userResp SCIMUserResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user response: %v", err)
	}
	
	// Verify the username in response
	if userResp.UserName != username {
		return nil, fmt.Errorf("username mismatch in response: expected %s, got %s", username, userResp.UserName)
	}
	
	// fmt.Printf("User '%s' created successfully for tenant %d with SCIM ID: %s\n", username, tenantIndex, userResp.ID)
	
	return &userResp, nil
}
