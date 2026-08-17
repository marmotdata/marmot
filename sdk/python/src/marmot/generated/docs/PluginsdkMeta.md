# PluginsdkMeta


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**category** | **str** |  | [optional] 
**config_spec** | [**List[PluginsdkConfigField]**](PluginsdkConfigField.md) |  | [optional] 
**description** | **str** |  | [optional] 
**features** | **List[str]** |  | [optional] 
**icon** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**status** | **str** |  | [optional] 
**supports_data_preview** | **bool** |  | [optional] 

## Example

```python
from marmot.generated.models.pluginsdk_meta import PluginsdkMeta

# TODO update the JSON string below
json = "{}"
# create an instance of PluginsdkMeta from a JSON string
pluginsdk_meta_instance = PluginsdkMeta.from_json(json)
# print the JSON string representation of the object
print(PluginsdkMeta.to_json())

# convert the object into a dict
pluginsdk_meta_dict = pluginsdk_meta_instance.to_dict()
# create an instance of PluginsdkMeta from a dict
pluginsdk_meta_from_dict = PluginsdkMeta.from_dict(pluginsdk_meta_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


