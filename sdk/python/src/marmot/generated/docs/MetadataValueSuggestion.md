# MetadataValueSuggestion


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**count** | **int** |  | [optional] 
**example** | [**Asset**](Asset.md) |  | [optional] 
**value** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.metadata_value_suggestion import MetadataValueSuggestion

# TODO update the JSON string below
json = "{}"
# create an instance of MetadataValueSuggestion from a JSON string
metadata_value_suggestion_instance = MetadataValueSuggestion.from_json(json)
# print the JSON string representation of the object
print(MetadataValueSuggestion.to_json())

# convert the object into a dict
metadata_value_suggestion_dict = metadata_value_suggestion_instance.to_dict()
# create an instance of MetadataValueSuggestion from a dict
metadata_value_suggestion_from_dict = MetadataValueSuggestion.from_dict(metadata_value_suggestion_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


