# Desgin

The firedoor aims to offer the 3 following scenarios with the following edge cases

1) A unlimited expiration of a role that does not expire

If the schedule is empty, it assumes the startdate is on creation with unlimited duration

```yaml
spec:
    approval:
      required: false
    justification: Standard maintenance window
    policy:
    - namespace: defa
      rules:
      - apiGroups:
        - ""
        resources:
        - pods
        - services
        verbs:
        - get
        - list
        - watch
      - apiGroups:
        - ""
        resourceNames:
        - app-config
        - debug-config
        resources:
        - configmaps
        verbs:
        - get
        - list
        - create
        - update
        - patch
        - delete
    - namespace: kube-system
      rules:
      - apiGroups:
        - ""
        resources:
        - pods
        - services
        verbs:
        - get
        - list
        - watch
    schedule: {}
    subjects:
    - apiGroup: rbac.authorization.k8s.io
      kind: User
      name: mail@matthewmcleod.co.uk
    ticketID: PROD-1235
```

If the schedule is not empty and the startdate is on creation with unlimited duration is at the startdate, The startdate cannot be defined in the past

```yaml
    schedule: 
        start: "2025-10-18T22:13:07Z"
```

2) The schedule defines the rbacs that are created repetatedly according to the cron job.

If the cron is started without the startdate, it assums the current startdate and begines the clock immediately

```yaml
    schedule: 
        cron: "0 2 * * 6,0"
```

3) The schdule has a cron, but the startdate is in the future

The corn should be ignored until the given furture start date has passed. If in the past it should fail. The duration has is not given then it is invalid

```yaml
    schedule: 
        startdate:  "2025-10-18T22:13:07Z"
        cron: "0 2 * * 6,0"
```

the duration must be less than the cron seperation it should fail

```yaml
schedule:
        startdate:  "2025-10-18T22:13:07Z"
        cron: "0 2 * * 6,0" 
        duration: 720
```

4) The duration is created,
