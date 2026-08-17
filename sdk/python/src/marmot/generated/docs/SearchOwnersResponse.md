# SearchOwnersResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**owners** | [**List[TeamOwner]**](TeamOwner.md) |  | [optional] 

## Example

```python
from marmot.generated.models.search_owners_response import SearchOwnersResponse

# TODO update the JSON string below
json = "{}"
# create an instance of SearchOwnersResponse from a JSON string
search_owners_response_instance = SearchOwnersResponse.from_json(json)
# print the JSON string representation of the object
print(SearchOwnersResponse.to_json())

# convert the object into a dict
search_owners_response_dict = search_owners_response_instance.to_dict()
# create an instance of SearchOwnersResponse from a dict
search_owners_response_from_dict = SearchOwnersResponse.from_dict(search_owners_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


