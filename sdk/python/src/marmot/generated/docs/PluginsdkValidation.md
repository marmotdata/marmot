# PluginsdkValidation


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**max** | **int** |  | [optional] 
**max_len** | **int** |  | [optional] 
**min** | **int** |  | [optional] 
**min_len** | **int** |  | [optional] 
**pattern** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.pluginsdk_validation import PluginsdkValidation

# TODO update the JSON string below
json = "{}"
# create an instance of PluginsdkValidation from a JSON string
pluginsdk_validation_instance = PluginsdkValidation.from_json(json)
# print the JSON string representation of the object
print(PluginsdkValidation.to_json())

# convert the object into a dict
pluginsdk_validation_dict = pluginsdk_validation_instance.to_dict()
# create an instance of PluginsdkValidation from a dict
pluginsdk_validation_from_dict = PluginsdkValidation.from_dict(pluginsdk_validation_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


