package provider

import (
	"fmt"
	"strconv"
	"strings"

	uptimerobotapi "github.com/bartekbp/terraform-provider-uptimerobot/internal/provider/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceMonitor() *schema.Resource {
	return &schema.Resource{
		Create: resourceMonitorCreate,
		Read:   resourceMonitorRead,
		Update: resourceMonitorUpdate,
		Delete: resourceMonitorDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"friendly_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"url": {
				Type:     schema.TypeString,
				Required: true,
			},
			"type": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice(uptimerobotapi.MonitorType, false),
			},
			"sub_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(uptimerobotapi.MonitorSubType, false),
				// required for port monitoring
			},
			"port": {
				Type:     schema.TypeInt,
				Optional: true,
				// required for port monitoring
			},
			"keyword_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(uptimerobotapi.MonitorKeywordType, false),
				// required for keyword monitoring
			},
			"keyword_value": {
				Type:     schema.TypeString,
				Optional: true,
				// required for keyword monitoring
			},
			"interval": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  300,
			},
			"http_method": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(uptimerobotapi.MonitorHTTPMethod, false),
			},
			"http_username": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"http_password": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"http_auth_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(uptimerobotapi.MonitorHTTPAuthType, false),
			},
			"post_value": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"post_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(uptimerobotapi.MonitorPostType, false),
			},
			"post_content_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(uptimerobotapi.MonitorPostContentType, false),
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"ignore_ssl_errors": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"alert_contact": {
				Type:     schema.TypeSet,
				Optional: true,
				// PromoteSingle: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Required: true,
						},
						"threshold": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"recurrence": {
							Type:     schema.TypeInt,
							Optional: true,
						},
					},
				},
			},
			"custom_http_headers": {
				Type:     schema.TypeMap,
				Optional: true,
			},
			// TODO - mwindows
		},
	}
}

func resourceMonitorCreate(d *schema.ResourceData, m interface{}) error {
	req := uptimerobotapi.MonitorCreateRequest{
		FriendlyName: d.Get("friendly_name").(string),
		URL:          d.Get("url").(string),
		Type:         d.Get("type").(string),
	}

	switch req.Type {
	case "port":
		req.SubType = d.Get("sub_type").(string)
		req.Port = d.Get("port").(int)
		break
	case "keyword":
		req.KeywordType = d.Get("keyword_type").(string)
		req.KeywordValue = d.Get("keyword_value").(string)

		if method := d.Get("http_method").(string); len(method) > 0 {
			req.HTTPMethod = method
		} else {
			req.HTTPMethod = "GET"
		}

		req.HTTPUsername = d.Get("http_username").(string)
		req.HTTPPassword = d.Get("http_password").(string)
		req.HTTPAuthType = d.Get("http_auth_type").(string)
		break
	case "http":
		if method := d.Get("http_method").(string); len(method) > 0 {
			req.HTTPMethod = method
		} else {
			req.HTTPMethod = "GET"
		}

		req.HTTPUsername = d.Get("http_username").(string)
		req.HTTPPassword = d.Get("http_password").(string)
		req.HTTPAuthType = d.Get("http_auth_type").(string)
		req.PostValue = d.Get("post_value").(string)
		req.PostType = d.Get("post_type").(string)
		req.PostContentType = d.Get("post_content_type").(string)
		break
	}

	// Add optional attributes
	req.Interval = d.Get("interval").(int)

	req.IgnoreSSLErrors = d.Get("ignore_ssl_errors").(bool)

	alertContacts := d.Get("alert_contact").(*schema.Set)

	req.AlertContacts = make([]uptimerobotapi.MonitorRequestAlertContact, alertContacts.Len())
	for k, v := range alertContacts.List() {
		req.AlertContacts[k] = uptimerobotapi.MonitorRequestAlertContact{
			ID:         v.(map[string]interface{})["id"].(string),
			Threshold:  v.(map[string]interface{})["threshold"].(int),
			Recurrence: v.(map[string]interface{})["recurrence"].(int),
		}
	}

	// custom_http_headers
	httpHeaderMap := d.Get("custom_http_headers").(map[string]interface{})
	req.CustomHTTPHeaders = make(map[string]string, len(httpHeaderMap))
	for k, v := range httpHeaderMap {
		req.CustomHTTPHeaders[k] = v.(string)
	}

	monitor, err := m.(uptimerobotapi.UptimeRobotApiClient).CreateMonitor(req)
	if err != nil {
		return err
	}
	d.SetId(fmt.Sprintf("%d", monitor.ID))
	if err := updateMonitorResource(d, monitor); err != nil {
		return err
	}
	return nil
}

func resourceMonitorRead(d *schema.ResourceData, m interface{}) error {
	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return err
	}

	monitor, err := m.(uptimerobotapi.UptimeRobotApiClient).GetMonitor(id)
	if err != nil {
		if strings.HasPrefix(err.Error(), "Monitor not found") {
			d.SetId("")
			return nil
		} else {
			return err
		}
	}
	if err := updateMonitorResource(d, monitor); err != nil {
		return err
	}
	return nil
}

func resourceMonitorUpdate(d *schema.ResourceData, m interface{}) error {
	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return err
	}

	req := uptimerobotapi.MonitorUpdateRequest{
		ID:           id,
		FriendlyName: d.Get("friendly_name").(string),
		URL:          d.Get("url").(string),
		Type:         d.Get("type").(string),
	}

	switch req.Type {
	case "port":
		req.SubType = d.Get("sub_type").(string)
		req.Port = d.Get("port").(int)
		break
	case "keyword":
		req.KeywordType = d.Get("keyword_type").(string)
		req.KeywordValue = d.Get("keyword_value").(string)

		if method := d.Get("http_method").(string); len(method) > 0 {
			req.HTTPMethod = method
		} else {
			req.HTTPMethod = "GET"
		}

		req.HTTPUsername = d.Get("http_username").(string)
		req.HTTPPassword = d.Get("http_password").(string)
		req.HTTPAuthType = d.Get("http_auth_type").(string)
		break
	case "http":
		if method := d.Get("http_method").(string); len(method) > 0 {
			req.HTTPMethod = method
		} else {
			req.HTTPMethod = "GET"
		}
		req.HTTPUsername = d.Get("http_username").(string)
		req.HTTPPassword = d.Get("http_password").(string)
		req.HTTPAuthType = d.Get("http_auth_type").(string)
		break
	}

	// Add optional attributes
	req.Interval = d.Get("interval").(int)
	req.IgnoreSSLErrors = d.Get("ignore_ssl_errors").(bool)

	req.AlertContacts = make([]uptimerobotapi.MonitorRequestAlertContact, d.Get("alert_contact").(*schema.Set).Len())
	for k, v := range d.Get("alert_contact").(*schema.Set).List() {
		req.AlertContacts[k] = uptimerobotapi.MonitorRequestAlertContact{
			ID:         v.(map[string]interface{})["id"].(string),
			Threshold:  v.(map[string]interface{})["threshold"].(int),
			Recurrence: v.(map[string]interface{})["recurrence"].(int),
		}
	}

	// custom_http_headers
	httpHeaderMap := d.Get("custom_http_headers").(map[string]interface{})
	req.CustomHTTPHeaders = make(map[string]string, len(httpHeaderMap))
	for k, v := range httpHeaderMap {
		req.CustomHTTPHeaders[k] = v.(string)
	}

	monitor, err := m.(uptimerobotapi.UptimeRobotApiClient).UpdateMonitor(req)
	if err != nil {
		return err
	}
	if err := updateMonitorResource(d, monitor); err != nil {
		return err
	}
	return nil
}

func resourceMonitorDelete(d *schema.ResourceData, m interface{}) error {
	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return err
	}

	err = m.(uptimerobotapi.UptimeRobotApiClient).DeleteMonitor(id)
	if err != nil {
		return err
	}
	return nil
}

func updateMonitorResource(d *schema.ResourceData, m uptimerobotapi.Monitor) error {
	d.Set("friendly_name", m.FriendlyName)
	d.Set("url", m.URL)
	d.Set("type", m.Type)
	d.Set("status", m.Status)
	d.Set("interval", m.Interval)

	d.Set("sub_type", m.SubType)
	d.Set("port", m.Port)

	d.Set("keyword_type", m.KeywordType)
	d.Set("keyword_value", m.KeywordValue)

	d.Set("http_username", m.HTTPUsername)
	d.Set("http_password", m.HTTPPassword)
	// PS: There seems to be a bug in the UR api as it never returns this value
	// d.Set("http_auth_type", m.HTTPAuthType)

	d.Set("ignore_ssl_errors", m.IgnoreSSLErrors)

	if err := d.Set("custom_http_headers", m.CustomHTTPHeaders); err != nil {
		return fmt.Errorf("error setting custom_http_headers for resource %s: %s", d.Id(), err)
	}

	rawContacts := make([]map[string]interface{}, len(m.AlertContacts))
	for k, v := range m.AlertContacts {
		rawContacts[k] = map[string]interface{}{
			"id":         v.ID,
			"recurrence": v.Recurrence,
			"threshold":  v.Threshold,
		}
	}
	if err := d.Set("alert_contact", rawContacts); err != nil {
		return fmt.Errorf("error setting alert_contact for resource %s: %s", d.Id(), err)
	}

	return nil
}
