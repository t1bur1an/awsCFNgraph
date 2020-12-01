#awsCFNgraph

####Description

A simple tool to make graph of import/export values in cloudformation.

Create png graph with stack names and names of variables.

####Requirements for run

Compile with go1.15.1. I think it should compile with lower version, but I'm not test this.

External module requirements is:
* github.com/aws/aws-sdk-go
* github.com/goccy

Require two variables:
1. AWS_PROFILE
2. AWS_REGION

#####P.S.
Code is not so clean as should but working fine.