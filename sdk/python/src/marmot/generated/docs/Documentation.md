# Documentation


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**content** | **str** |  | [optional] 
**created_at** | **str** |  | [optional] 
**global_docs** | **List[str]** |  | [optional] 
**id** | **str** |  | [optional] 
**mrn** | **str** |  | [optional] 
**source** | **str** |  | [optional] 
**updated_at** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.documentation import Documentation

# TODO update the JSON string below
json = "{}"
# create an instance of Documentation from a JSON string
documentation_instance = Documentation.from_json(json)
# print the JSON string representation of the object
print(Documentation.to_json())

# convert the object into a dict
documentation_dict = documentation_instance.to_dict()
# create an instance of Documentation from a dict
documentation_from_dict = Documentation.from_dict(documentation_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


