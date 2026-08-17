# AWSCredentialStatus


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**available** | **bool** |  | [optional] 
**error** | **str** |  | [optional] 
**sources** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.aws_credential_status import AWSCredentialStatus

# TODO update the JSON string below
json = "{}"
# create an instance of AWSCredentialStatus from a JSON string
aws_credential_status_instance = AWSCredentialStatus.from_json(json)
# print the JSON string representation of the object
print(AWSCredentialStatus.to_json())

# convert the object into a dict
aws_credential_status_dict = aws_credential_status_instance.to_dict()
# create an instance of AWSCredentialStatus from a dict
aws_credential_status_from_dict = AWSCredentialStatus.from_dict(aws_credential_status_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


