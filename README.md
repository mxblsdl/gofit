# Go Fit

Experimental interface to my own fitbit data

Visualizing this data with echarts


## Hosting
- setting a static IP address on your server machine
- changing the local DNS settings on the server to point to `fitbit-pi.local`
  - sudo hostnamectl set-hostname fitbit-pi
- Setting the server to listen on all interfaces (0.0.0.0) instead of just localhost
  