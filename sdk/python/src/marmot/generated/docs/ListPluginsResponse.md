# ListPluginsResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**loading** | **bool** |  | [optional] 
**plugins** | [**List[PluginsdkMeta]**](PluginsdkMeta.md) |  | [optional] 

## Example

```python
from marmot.generated.models.list_plugins_response import ListPluginsResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ListPluginsResponse from a JSON string
list_plugins_response_instance = ListPluginsResponse.from_json(json)
# print the JSON string representation of the object
print(ListPluginsResponse.to_json())

# convert the object into a dict
list_plugins_response_dict = list_plugins_response_instance.to_dict()
# create an instance of ListPluginsResponse from a dict
list_plugins_response_from_dict = ListPluginsResponse.from_dict(list_plugins_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


