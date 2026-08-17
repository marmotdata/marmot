# PluginRun


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**completed_at** | **str** |  | [optional] 
**config** | **Dict[str, object]** |  | [optional] 
**created_by** | **str** |  | [optional] 
**error_message** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**pipeline_name** | **str** |  | [optional] 
**run_id** | **str** |  | [optional] 
**source_name** | **str** |  | [optional] 
**started_at** | **str** |  | [optional] 
**status** | [**RunStatus**](RunStatus.md) |  | [optional] 
**summary** | [**RunSummary**](RunSummary.md) |  | [optional] 

## Example

```python
from marmot.generated.models.plugin_run import PluginRun

# TODO update the JSON string below
json = "{}"
# create an instance of PluginRun from a JSON string
plugin_run_instance = PluginRun.from_json(json)
# print the JSON string representation of the object
print(PluginRun.to_json())

# convert the object into a dict
plugin_run_dict = plugin_run_instance.to_dict()
# create an instance of PluginRun from a dict
plugin_run_from_dict = PluginRun.from_dict(plugin_run_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


