# ListSSOMappingsResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**mappings** | [**List[SSOTeamMapping]**](SSOTeamMapping.md) |  | [optional] 

## Example

```python
from marmot.generated.models.list_sso_mappings_response import ListSSOMappingsResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ListSSOMappingsResponse from a JSON string
list_sso_mappings_response_instance = ListSSOMappingsResponse.from_json(json)
# print the JSON string representation of the object
print(ListSSOMappingsResponse.to_json())

# convert the object into a dict
list_sso_mappings_response_dict = list_sso_mappings_response_instance.to_dict()
# create an instance of ListSSOMappingsResponse from a dict
list_sso_mappings_response_from_dict = ListSSOMappingsResponse.from_dict(list_sso_mappings_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


