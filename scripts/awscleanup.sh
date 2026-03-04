#!/bin/bash

# DEPRECATED: This script has been replaced by the 'holodeck cleanup' command.
# Please use: holodeck cleanup <vpc-id> [vpc-id...]
# This script will be removed in a future release.

echo "WARNING: This script is deprecated. Please use 'holodeck cleanup' command instead." >&2

if [[ $# -ne 1 ]]; then
    echo " vpcid required for deletion"
    exit 1
fi

export vpcid=$1

get_tag_value(){
    if [[ $# -ne 2 ]]; then
        echo " vpcid and key required to get tag value"
        exit 1
    fi
    local vpc=$1
    local key=$2
    aws ec2 describe-tags --filters "Name=resource-id,Values=$vpcid" "Name=key,Values=$key" \
        --query "Tags[0].Value" --output text
}

delete_vpc_resources() {
    if [[ $# -ne 1 ]]; then
        echo " vpcid required for deletion"
        exit 1
    fi
    local vpcid=$1

    echo "Start cleanup of resources in  VPC: $vpcid"

    # Delete Instance
    instances=$(aws ec2 describe-instances \
        --filters "Name=vpc-id,Values=$vpcid" \
        --query "Reservations[].Instances[].InstanceId" \
        --output text | tr -d '\r' | tr '\n' ' ')
    for instance in $instances; do
        aws ec2 terminate-instances --instance-ids "$instance"
    done

    # Detach and Delete Security Groups
    security_groups=$(aws ec2 describe-security-groups \
        --filters Name=vpc-id,Values=$vpcid \
        --query "SecurityGroups[?GroupName!='default'].GroupId" \
        --output text | tr -d '\r' | tr '\n' ' ')
    for sg in $security_groups; do
        enis=$(aws ec2 describe-network-interfaces \
            --filters Name=group-id,Values=$sg \
            --query "NetworkInterfaces[].NetworkInterfaceId" \
            --output text | tr -d '\r' | tr '\n' ' ')
        for eni in $enis; do
            aws ec2 modify-network-interface-attribute \
                --network-interface-id "$eni" \
                --groups "$(aws ec2 describe-security-groups \
                    --query 'SecurityGroups[?GroupName==`default`].GroupId' \
                    --output text)"
        done
        aws ec2 delete-security-group --group-id "$sg"
    done

    # Delete Subnets
    subnets=$(aws ec2 describe-subnets \
        --filters Name=vpc-id,Values=$vpcid \
        --query "Subnets[].SubnetId" \
        --output text | tr -d '\r' | tr '\n' ' ')
    for subnet in $subnets; do
        aws ec2 delete-subnet --subnet-id "$subnet"
    done

    # Delete Route Tables
    # 1. Make first rt as Main , as we cannot delete vpcs attached with main
    # 2. replace all rt with first rt
    # 3. delete rt
    # 4. Main table(first_rt) will be deleted once vpc deleted
    first_rt=""
    route_tables=$(aws ec2 describe-route-tables \
        --filters Name=vpc-id,Values=$vpcid \
        --query "RouteTables[].RouteTableId" \
        --output text | tr -d '\r' | tr '\n' ' ')
    for rt in $route_tables; do
        associations=$(aws ec2 describe-route-tables \
                --route-table-ids "$rt" \
                --query "RouteTables[].Associations[].RouteTableAssociationId" \
                --output text | tr -d '\r' | tr '\n' ' ')
        for assoc_id in $associations; do
            if [ -z "$first_rt" ]; then
                aws ec2 replace-route-table-association --association-id $assoc_id --route-table-id $rt
                first_rt=$rt
            else
                aws ec2 replace-route-table-association --association-id $assoc_id --route-table-id $first_rt
            fi
        done
        aws ec2 delete-route-table --route-table-id "$rt" 2>>/dev/null
    done

    # Delete Internet Gateway
    internet_gateways=$(aws ec2 describe-internet-gateways \
        --filters Name=attachment.vpc-id,Values=$vpcid \
        --query "InternetGateways[].InternetGatewayId" \
        --output text | tr -d '\r' | tr '\n' ' ')
    for igw in $internet_gateways; do
        aws ec2 detach-internet-gateway --internet-gateway-id "$igw" --vpc-id "$vpcid"
        aws ec2 delete-internet-gateway --internet-gateway-id "$igw"
    done

    # Delete vpc
    # try 3 times with 30 seconds interval
    attempts=0
    echo "All resource Deleted for VPC: $vpcid , now delete vpc"
    while [ $attempts -lt 3 ]; do
        if aws ec2 delete-vpc --vpc-id $vpcid; then
            echo "Successfully deleted VPC: $vpcid"
            break
        else
            attempts=$((attempts + 1))
            if [ $attempts -lt 3 ]; then
                echo "Failed to delete VPC: $vpcid. Retrying in 30 seconds..."
                sleep 30
            fi
        fi
    done
    if [ $attempts -eq 3 ]; then
        echo "Failed to delete VPC: $vpcid after 3 attempts. Continue the loop to delete other vpc"
    fi
}

github_repository=$(get_tag_value $vpcid "GitHubRepository")
run_id=$(get_tag_value $vpcid "GitHubRunId")
job_name=$(get_tag_value $vpcid "GitHubJob")
response=$(curl -s -H "Authorization: Bearer $GITHUB_TOKEN" \
    "https://api.github.com/repos/${github_repository}/actions/runs/${run_id}/jobs")
if [[ -z "$response" || "$response" == "null" ]]; then
    exit 0
fi

# 1. make sure .jobs exist in response
# e.g. { "message": "Not Found", "documentation_url": "https://docs.github.com/rest", "status": "404" }
# 2. check if all jobs completed

if ! echo "$response" | jq -e '.jobs != null' >/dev/null 2>&1; then
    exit 0
fi

is_jobs_not_completed=$(echo "$response" | jq -r ".jobs? // [] |
    map(select(.status != \"completed\")) |
    length")

if [[ "$is_jobs_not_completed" -eq 0 ]]; then
    echo "Holodeck e2e Job status is not in running stage , Delete the vpc $vpcid and dependent resources"
    delete_vpc_resources $vpcid
fi
