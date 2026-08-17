# AvailableFilters


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**providers** | **Dict[str, int]** |  | [optional] 
**tags** | **Dict[str, int]** |  | [optional] 
**types** | **Dict[str, int]** |  | [optional] 

## Example

```python
from marmot.generated.models.available_filters import AvailableFilters

# TODO update the JSON string below
json = "{}"
# create an instance of AvailableFilters from a JSON string
available_filters_instance = AvailableFilters.from_json(json)
# print the JSON string representation of the object
print(AvailableFilters.to_json())

# convert the object into a dict
available_filters_dict = available_filters_instance.to_dict()
# create an instance of AvailableFilters from a dict
available_filters_from_dict = AvailableFilters.from_dict(available_filters_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


