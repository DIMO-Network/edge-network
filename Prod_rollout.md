# Production Rollout Process

**It is important not to apply updates to all devices at once, as this can cause issues if the code has bugs!**

**It's essential to avoid making a release on Fridays and Holidays!!! If something goes wrong, you don't want to be stuck fixing it over the weekend.**

## Release to few and Validate

Let's assume your code is all good and you wanna update all the Autopi's in our fleet of devices. 
First we validate your production build we work and be applied correctly.

1. Make a Release in Github from main branch in edge-network repo, all dev work should be merged in.
   Release name should be semver v0.0.0 and must not end in 0. 
   Newest release should be interpreted as a higher release than the last one.

2. Pick a few devices, ideally ones you have some control over. 
3. Apply the update individually to the device using the Autopi cloud. Do NOT update the template yet.

### Validation Steps

1. Check that the update only gets downloaded once, and is installed successfully. Best way is using the twilio index in kibana:
   [Example Query](https://kibana.team.dimo.zone/app/discover#/?_g=(filters:!(),refreshInterval:(pause:!t,value:60000),time:(from:now-24h%2Fh,to:now))&_a=(columns:!(data.event_type,data.data_download),filters:!(),hideChart:!f,index:c100d670-a496-11ec-a159-9f3770acfafe,interval:auto,query:(language:kuery,query:'data.imei%20:%20%22353338970432065%22%20and%20data.data_download%20%3E%201000000'),sort:!(!(time,desc))))
   `data.imei : "353338970432065" and data.data_download > 1000000`. Be sure to watch it over a period of time
   to make sure doesn't keep downloading same update over and over again.
   To check version on device, from terminal in AP Cloud: `cmd.run 'edge-network -v'`
2. Check that mqtt data is still being sent, can use kibana or clickhouse.
3. Monitor the [Grafana dashboards](https://grafana.team.dimo.zone/d/fdq1u88iocjy8b/v2-status-pipeline?var-environment=prod&orgId=1&from=now-6h&to=now) for any anomalies. 

## How to update all devices, once ready:

You've passed all the validation steps and are ready to rollout to all devices:
 
1. Update AutoPi Cloud template 130 (DIMO Device Client), under DIMO section set the latest binary edge-network version.
2. In the Devices screen under the template, use the Apply All Async button and wait for all to complete.

# Release costs

Example math:
7.8 MB x ~11500 devices = 89.7 GB lte transfer.
Twilio costs seems to be about $11.3 / Gb => $1,020.

So every time we release a new edge-network binary it costs us about $1k, given the number of devices as of Aug-2024