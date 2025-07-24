# IP Detection Guide

## Overview

Holodeck automatically detects your public IP address when creating AWS
environments, eliminating the need to manually configure security group rules.

## How It Works

### Detection Process

1. **Service Priority**: Tries multiple IP detection services in order
1. **Fallback Strategy**: If one service fails, automatically tries the next
1. **Validation**: Ensures detected IP is a valid public IPv4 address
1. **CIDR Formatting**: Automatically adds `/32` suffix for AWS compatibility

### Supported Services

- `https://api.ipify.org?format=text` (Primary)
- `https://ifconfig.me/ip` (Fallback 1)
- `https://icanhazip.com` (Fallback 2)
- `https://ident.me` (Fallback 3)

### Timeout Configuration

- **Overall Timeout**: 15 seconds
- **Per-Service Timeout**: 5 seconds
- **Context Support**: Proper cancellation and timeout handling

## Configuration Examples

### Basic Usage (Recommended)

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: my-environment
spec:
  provider: aws
  instance:
    type: g4dn.xlarge
    region: us-west-2
    # No ingressIpRanges needed - IP detected automatically
```

### With Additional IP Ranges

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: my-environment
spec:
  provider: aws
  instance:
    type: g4dn.xlarge
    region: us-west-2
    ingressIpRanges:
      - "10.0.0.0/8"      # Corporate network
      - "172.16.0.0/12"   # Additional network
    # Your detected IP will be automatically added
```

## Troubleshooting

### Common Issues

1. **Network Connectivity**: Ensure outbound internet access to IP detection services
1. **Firewall Rules**: Corporate firewalls may block IP detection services
1. **Proxy Configuration**: Proxy settings may affect IP detection

### Manual Override

If automatic detection fails, you can manually specify your IP:

```yaml
spec:
  provider: aws
  instance:
    type: g4dn.xlarge
    region: us-west-2
    ingressIpRanges:
      - "YOUR_PUBLIC_IP/32"  # Replace with your actual public IP
```

### Debugging

To debug IP detection issues:

```bash
# Test IP detection manually
curl https://api.ipify.org?format=text
curl https://ifconfig.me/ip
curl https://icanhazip.com
curl https://ident.me
```

## Security Considerations

### IP Validation

The system validates that detected IPs are:

- Valid IPv4 addresses
- Public (not private, loopback, or link-local)
- Properly formatted for AWS security groups

### Network Security

- Only your current public IP is granted access
- Additional IP ranges can be specified manually
- Security group rules are automatically configured

## Best Practices

1. **Use Automatic Detection**: Let Holodeck handle IP detection automatically
1. **Specify Additional Ranges**: Use `ingressIpRanges` only for additional networks
1. **Test Connectivity**: Verify access to IP detection services in your environment
1. **Monitor Changes**: Be aware that your public IP may change (DHCP, mobile networks)

## Related Documentation

- [Create Command](../commands/create.md#automated-ip-detection)
- [Prerequisites](../prerequisites.md#network-requirements)
- [Examples](../../examples/README.md#updated-aws-examples)
