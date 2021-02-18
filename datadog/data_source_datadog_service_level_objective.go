package datadog

import (
	"fmt"

	datadogV1 "github.com/DataDog/datadog-api-client-go/api/v1/datadog"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/terraform-providers/terraform-provider-datadog/datadog/internal/utils"
)

func dataSourceDatadogServiceLevelObjective() *schema.Resource {
	return &schema.Resource{
		Description: "Use this data source to retrieve information about an existing monitor for use in other resources.",
		Read:        dataSourceServiceLevelObjectiveRead,
		Schema: map[string]*schema.Schema{
			"name_query": {
				Description: "The query string to filter results based on SLO names.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"tags_filter": {
				Description: "The query string to filter results based on a single SLO tag.",
				Type:        schema.TypeString,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"metrics_query": {
				Description: "The query string to filter results based on SLO numerator and denominator.",
				Type:        schema.TypeString,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},

			// Computed values
			"name": {
				Description: "Name of Datadog service level objective",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"description": {
				Description: "A description of this service level objective.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"tags": {
				Type:        schema.TypeSet,
				Description: "A list of tags to associate with your service level objective. This can help you categorize and filter service level objectives in the service level objectives page of the UI. Note: it's not currently possible to filter by these tags when querying via the API",
				Elem:        &schema.Schema{Type: schema.TypeString},
				Computed:    true,
			},
			"thresholds": {
				Description: "A list of thresholds and targets that define the service level objectives from the provided SLIs.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"timeframe": {
							Description: "The time frame for the objective. The mapping from these types to the types found in the Datadog Web UI can be found in the Datadog API documentation page. Available options to choose from are: `7d`, `30d`, `90d`.",
							Type:        schema.TypeString,
							Computed:    true,
						},
						"target": {
							Description: "The objective's target in`[0,100]`.",
							Type:        schema.TypeFloat,
							Computed:    true,
						},
						"target_display": {
							Description: "A string representation of the target that indicates its precision. It uses trailing zeros to show significant decimal places (e.g. `98.00`).",
							Type:        schema.TypeString,
							Computed:    true,
						},
						"warning": {
							Description: "The objective's warning value in `[0,100]`. This must be greater than the target value.",
							Type:        schema.TypeFloat,
							Computed:    true,
						},
						"warning_display": {
							Description: "A string representation of the warning target (see the description of the target_display field for details).",
							Type:        schema.TypeString,
							Computed:    true,
						},
					},
				},
			},
			"type": {
				Description: "The type of the service level objective. The mapping from these types to the types found in the Datadog Web UI can be found in the Datadog API [documentation page](https://docs.datadoghq.com/api/v1/service-level-objectives/#create-a-slo-object). Available options to choose from are: `metric` and `monitor`.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"force_delete": {
				Description: "A boolean indicating whether this monitor can be deleted even if it’s referenced by other resources (e.g. dashboards).",
				Type:        schema.TypeBool,
				Computed:    true,
			},
			// Metric-Based SLO
			"query": {
				Type:        schema.TypeList,
				MaxItems:    1,
				Computed:    true,
				Description: "The metric query of good / total events",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"numerator": {
							Description: "The sum of all the `good` events.",
							Type:        schema.TypeString,
							Computed:    true,
						},
						"denominator": {
							Description: "The sum of the `total` events.",
							Type:        schema.TypeString,
							Computed:    true,
						},
					},
				},
			},
			// Monitor-Based SLO
			"monitor_ids": {
				Type:        schema.TypeSet,
				Computed:    true,
				Description: "A static set of monitor IDs to use as part of the SLO",
				Elem:        &schema.Schema{Type: schema.TypeInt, MinItems: 1},
			},
			"monitor_search": {
				Type:     schema.TypeString,
				Removed:  "Feature is not yet supported",
				Computed: true,
			},
			"groups": {
				Type:        schema.TypeSet,
				Computed:    true,
				Description: "A static set of groups to filter monitor-based SLOs",
				Elem:        &schema.Schema{Type: schema.TypeString, MinItems: 1},
			},
			"validate": {
				Description: "Whether or not to validate the SLO.",
				Type:        schema.TypeBool,
				Computed:    true,
			},
		},
	}
}

func dataSourceServiceLevelObjectiveRead(d *schema.ResourceData, meta interface{}) error {
	providerConf := meta.(*ProviderConfiguration)
	datadogClientV1 := providerConf.DatadogClientV1
	authV1 := providerConf.AuthV1

	req := datadogClientV1.ServiceLevelObjectivesApi.ListSLOs(authV1)
	if v, ok := d.GetOk("name_query"); ok {
		req = req.Query(v.(string))
	}
	if v, ok := d.GetOk("tags_filter"); ok {
		req = req.TagsQuery(v.(string))
	}
	if v, ok := d.GetOk("metrics_query"); ok {
		req = req.MetricsQuery(v.(string))
	}

	sloResponse, _, err := req.Execute()
	if err != nil {
		return utils.TranslateClientError(err, "error querying monitors")
	}

	slos := sloResponse.GetData()
	if len(slos) > 1 {
		return fmt.Errorf("your query returned more than one result, please try a more specific search criteria")
	}
	if len(slos) == 0 {
		return fmt.Errorf("your query returned no result, please try a less specific search criteria")
	}

	slo := slos[0]

	thresholds := make([]map[string]interface{}, 0)
	for _, threshold := range slo.GetThresholds() {
		t := map[string]interface{}{
			"timeframe": threshold.GetTimeframe(),
			"target":    threshold.GetTarget(),
		}
		if warning, ok := threshold.GetWarningOk(); ok {
			t["warning"] = warning
		}
		if targetDisplay, ok := threshold.GetTargetDisplayOk(); ok {
			t["target_display"] = targetDisplay
		}
		if warningDisplay, ok := threshold.GetWarningDisplayOk(); ok {
			t["warning_display"] = warningDisplay
		}
		thresholds = append(thresholds, t)
	}

	tags := make([]string, 0)
	for _, s := range slo.GetTags() {
		tags = append(tags, s)
	}

	if err := d.Set("name", slo.GetName()); err != nil {
		return err
	}
	if err := d.Set("description", slo.GetDescription()); err != nil {
		return err
	}
	if err := d.Set("type", slo.GetType()); err != nil {
		return err
	}
	if err := d.Set("tags", tags); err != nil {
		return err
	}
	if err := d.Set("thresholds", thresholds); err != nil {
		return err
	}
	switch slo.GetType() {
	case datadogV1.SLOTYPE_MONITOR:
		// monitor type
		if len(slo.GetMonitorIds()) > 0 {
			if err := d.Set("monitor_ids", slo.GetMonitorIds()); err != nil {
				return err
			}
		}
		if err := d.Set("groups", slo.GetGroups()); err != nil {
			return err
		}
	default:
		// metric type
		query := make(map[string]interface{})
		q := slo.GetQuery()
		query["numerator"] = q.GetNumerator()
		query["denominator"] = q.GetDenominator()
		if err := d.Set("query", []map[string]interface{}{query}); err != nil {
			return err
		}
	}
	d.SetId(slo.GetId())

	return nil
}
