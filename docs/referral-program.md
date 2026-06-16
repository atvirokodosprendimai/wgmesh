# wgmesh Referral Program

## Overview

The wgmesh referral program rewards customers for sharing wgmesh with their team and network. When someone creates an account using your referral code and completes their first mesh setup, you earn rewards.

## Getting Started

### Create an Account

If you don't already have an account, create one:

```bash
wgmesh account create --email your@email.com
```

You'll receive a unique referral code that you can share with others.

### Check Your Status

View your account details, referral code, and rewards:

```bash
wgmesh account status
```

Example output:

```
Account Details:
  Account ID: 550e8400-e29b-41d4-a716-446655440000
  Email: user@example.com
  Created: 2026-06-16

Your Referral Code:
  ABC123XYZ4A

  Share this code with your team to earn rewards!

Referral Stats:
  Total Referrals: 7
  Converted: 5 (71.4%)

Pending Rewards:
  - 1 Month Service Credit
  - 3-Month Service Extension
```

## How Referrals Work

1. **Share your code**: Give your referral code to friends, colleagues, or your team
2. **They create an account**: They use `wgmesh account create --referral-code YOURCODE`
3. **They set up their mesh**: When they complete their first mesh setup, the referral is recorded
4. **You earn rewards**: Once their mesh is running, you earn rewards based on your total referral count

### Using a Referral Code

If someone referred you to wgmesh, use their code when creating your account:

```bash
wgmesh account create --email your@email.com --referral-code ABC123XYZ4A
```

## Reward Structure

Rewards are earned based on the number of successful referrals (referrals that complete mesh setup):

| Referrals | Reward | Description |
|-----------|--------|-------------|
| 1 | 1 Month Service Credit | One month of service credit |
| 5 | 3-Month Service Extension | Three months of free service |
| 10 | Premium Feature Unlock | Access to premium features |

Rewards are cumulative - reaching 10 referrals unlocks all three reward tiers.

### Viewing Your Referrals

See all your referrals and their conversion status:

```bash
wgmesh account referrals
```

Example output:

```
Your Referrals (5):

| Referred ID                               | Code Used   | Converted At |
|------------------------------------------|-------------|--------------|
| 6ba7b810-9dad-11d1-80b4-00c04fd430c8     | ABC123XYZ4A | 2026-06-15   |
| 7fd9c3b1-2a4e-4f5d-8e1a-9b3c7d8e9f0a     | ABC123XYZ4A | 2026-06-14   |
| 9e8d7c6b-5a4f-3e2d-1c0b-9a8b7c6d5e4f     | ABC123XYZ4A | Not yet      |
```

## Referral Code Format

Referral codes are:
- **12 characters long**
- **URL-safe** (uppercase letters and numbers only)
- **Typo-resistant** (includes checksum to detect errors)
- **Case-insensitive** (though displayed in uppercase)

Example: `ABC123XYZ4A`

## FAQ

### Can I refer my team?
Yes! Referring multiple users from the same organization is allowed and encouraged. Each team member who creates an account with your referral code counts as a separate referral.

### When do rewards apply?
Rewards are earned when a referred account completes their first successful mesh setup. Simply creating an account isn't enough - the mesh must be operational.

### Can I use multiple referral codes?
No, each account can only use one referral code during creation. Choose the referrer whose code you want to support.

### Do referral codes expire?
Currently, referral codes do not expire. As long as the code format remains valid, it can be used.

### What happens if I enter an invalid referral code?
The system will validate the code format and check if it exists. If the code is invalid or not found, you'll receive an error message and can create an account without a referral code.

### How is conversion rate calculated?
Conversion rate = (Number of referred accounts with completed mesh setups) / (Total number of referrals) × 100%

For example, if 5 out of 7 referred accounts have completed mesh setup:
- Conversion rate = 5/7 × 100% = 71.4%

### Can I change my email address later?
Currently, email addresses are set at account creation and cannot be modified through the CLI. Contact support if you need to update your email.

## Best Practices

### Sharing Your Referral Code

**Do:**
- Share your code with team members who would benefit from wgmesh
- Include your code in team documentation or onboarding materials
- Share your experience with wgmesh along with your code

**Don't:**
- Post your code publicly in forums or social media (unless appropriate)
- Spam your code to unsolicited recipients
- Misrepresent the referral program or rewards

### Tracking Your Progress

Check your referral status regularly:

```bash
wgmesh account status
```

This shows:
- Your referral code (for easy sharing)
- Total referrals
- Conversion rate
- Pending rewards

## Technical Details

### Account Storage

Account data is stored locally at `~/.wgmesh/accounts.json` in JSON format. The store includes:
- Your account ID and email
- Your referral code
- All referrals you've made
- Conversion status of each referral

### Privacy

Referral codes are:
- Derived from your account ID using cryptographic functions
- Include checksums for error detection
- Do not contain any personal information

The referral program does not track:
- What meshes are created
- Mesh configuration details
- Network traffic or usage

Only referral relationships and conversion status are tracked.

## Support

If you encounter issues with the referral program:

1. Check that your referral code is valid: `wgmesh account status`
2. Verify the code format (12 characters, alphanumeric)
3. Ensure the person you referred used your exact code

For additional support, refer to the main wgmesh documentation.
