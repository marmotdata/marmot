# FacetValue


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**count** | **int** |  | [optional] 
**value** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.facet_value import FacetValue

# TODO update the JSON string below
json = "{}"
# create an instance of FacetValue from a JSON string
facet_value_instance = FacetValue.from_json(json)
# print the JSON string representation of the object
print(FacetValue.to_json())

# convert the object into a dict
facet_value_dict = facet_value_instance.to_dict()
# create an instance of FacetValue from a dict
facet_value_from_dict = FacetValue.from_dict(facet_value_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


