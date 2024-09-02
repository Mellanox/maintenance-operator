#!/bin/bash
# This script ad-hoc fixes references in api doc

DOC=$(realpath $1)

awk '
{
    gsub(/\(#maintenance.nvidia.com%2fv1alpha1\)/, "\(#maintenancenvidiacomv1alpha1\)");
    gsub(/#maintenance.nvidia.com\/v1alpha1./, "#");
    gsub(/#OperatorLogLevel/, "#OperatorLogLevel-string-alias")
}1
' ${DOC} > ${DOC}.tmp
mv -f ${DOC}.tmp ${DOC}
