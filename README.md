
# analyze-pt-stalk

It will turn collected MySQL variables values into nice graphical dashboards.
  
## how to run

run the script

```bash
$ go run main.go /path-to-pt-stalk/
Processing file: /path-to-pt-stalk/2024_09_09_12_46_02-mysqladmin
Processing file: /path-to-pt-stalk/2024_09_09_12_46_32-mysqladmin
2024/09/09 16:58:47 Server started. Go to http://localhost:8080
2024/09/09 16:58:51 Rendering template...
2024/09/09 16:58:52 Rendering template...
```

## notes

The code was Python first, and then I converted into golang to have easy install, more beautiful charts and so on. We still have python version in here but I won't update anymore.
  
