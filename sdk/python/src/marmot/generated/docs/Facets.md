# Facets


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_types** | [**List[FacetValue]**](FacetValue.md) |  | [optional] 
**providers** | [**List[FacetValue]**](FacetValue.md) |  | [optional] 
**tags** | [**List[FacetValue]**](FacetValue.md) |  | [optional] 
**types** | **Dict[str, int]** |  | [optional] 

## Example

```python
from marmot.generated.models.facets import Facets

# TODO update the JSON string below
json = "{}"
# create an instance of Facets from a JSON string
facets_instance = Facets.from_json(json)
# print the JSON string representation of the object
print(Facets.to_json())

# convert the object into a dict
facets_dict = facets_instance.to_dict()
# create an instance of Facets from a dict
facets_from_dict = Facets.from_dict(facets_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


