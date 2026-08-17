# PluginsdkConfigField


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**default** | **object** |  | [optional] 
**description** | **str** |  | [optional] 
**fields** | [**List[PluginsdkConfigField]**](PluginsdkConfigField.md) |  | [optional] 
**hidden** | **bool** |  | [optional] 
**is_array** | **bool** |  | [optional] 
**label** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**options** | [**List[PluginsdkFieldOption]**](PluginsdkFieldOption.md) |  | [optional] 
**placeholder** | **str** |  | [optional] 
**required** | **bool** |  | [optional] 
**sensitive** | **bool** |  | [optional] 
**show_when** | [**PluginsdkShowWhen**](PluginsdkShowWhen.md) |  | [optional] 
**type** | [**PluginsdkFieldType**](PluginsdkFieldType.md) |  | [optional] 
**validation** | [**PluginsdkValidation**](PluginsdkValidation.md) |  | [optional] 

## Example

```python
from marmot.generated.models.pluginsdk_config_field import PluginsdkConfigField

# TODO update the JSON string below
json = "{}"
# create an instance of PluginsdkConfigField from a JSON string
pluginsdk_config_field_instance = PluginsdkConfigField.from_json(json)
# print the JSON string representation of the object
print(PluginsdkConfigField.to_json())

# convert the object into a dict
pluginsdk_config_field_dict = pluginsdk_config_field_instance.to_dict()
# create an instance of PluginsdkConfigField from a dict
pluginsdk_config_field_from_dict = PluginsdkConfigField.from_dict(pluginsdk_config_field_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


