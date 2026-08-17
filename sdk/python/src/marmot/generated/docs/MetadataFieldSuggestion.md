# MetadataFieldSuggestion


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**count** | **int** |  | [optional] 
**example** | **object** |  | [optional] 
**var_field** | **str** |  | [optional] 
**path_parts** | **List[str]** |  | [optional] 
**type** | **str** |  | [optional] 
**types** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.metadata_field_suggestion import MetadataFieldSuggestion

# TODO update the JSON string below
json = "{}"
# create an instance of MetadataFieldSuggestion from a JSON string
metadata_field_suggestion_instance = MetadataFieldSuggestion.from_json(json)
# print the JSON string representation of the object
print(MetadataFieldSuggestion.to_json())

# convert the object into a dict
metadata_field_suggestion_dict = metadata_field_suggestion_instance.to_dict()
# create an instance of MetadataFieldSuggestion from a dict
metadata_field_suggestion_from_dict = MetadataFieldSuggestion.from_dict(metadata_field_suggestion_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


