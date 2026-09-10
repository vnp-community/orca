// src/relay/device-capabilities-handler.ts
// Backs the `device.capabilities` RPC — see
// specs/emulator/tdd/v1/02-device-rpc-catalog.md for the result shape.
import { platform } from 'node:os'
import { discoverAndroidSdk, type AndroidSdkInfo } from './device-android-discovery'
import { discoverIosToolchain, type IosToolchainInfo } from './device-ios-discovery'

export type DeviceCapabilities = {
  platform: NodeJS.Platform
  android: AndroidSdkInfo
  ios: IosToolchainInfo
}

export async function getDeviceCapabilities(androidSdkOverride?: string | null): Promise<DeviceCapabilities> {
  const android = discoverAndroidSdk(androidSdkOverride)
  const ios = await discoverIosToolchain()
  return { platform: platform(), android, ios }
}
