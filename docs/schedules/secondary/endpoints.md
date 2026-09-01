# Campus API Endpoint Reference (api.campus.kpi.ua)

All endpoints listed below are hosted on `https://api.campus.kpi.ua` and accept standard HTTP `GET` requests without authentication.

---

## 1. Academic Time & Slots

### 1.1 Get Current Academic Time
Returns the current active academic week (1 or 2), day of the week, and active lesson slot.

- **Method**: `GET`
- **URL**: `https://api.campus.kpi.ua/time/current`
- **Response**: `200 OK`
```json
{
  "currentWeek": 1,
  "currentDay": 2,
  "currentLesson": 1
}
```
*Note: `currentDay` is 1-indexed (1 = Monday, 2 = Tuesday, ..., 7 = Sunday).*

---

### 1.2 Get Lesson Time Slots
Returns the official schedule bells / pair start times.

- **Method**: `GET`
- **URL**: `https://api.campus.kpi.ua/schedule/lessons/slots`
- **Response**: `200 OK`
```json
{
  "1": "08:30:00",
  "2": "10:25:00",
  "3": "12:20:00",
  "4": "14:15:00",
  "5": "16:10:00",
  "6": "18:05:00",
  "7": "20:00:00"
}
```

---

## 2. Groups & Catalogs

### 2.1 Get All Academic Groups
Returns the full directory of university groups.

- **Method**: `GET`
- **URL**: `https://api.campus.kpi.ua/group/all` *(Redirects `302` to `https://cdn.cloud.kpi.ua/schedule-groups-ukrainian.json`)*
- **Response**: `200 OK`
```json
[
  {
    "id": 4402,
    "name": "ІП-21",
    "faculty": "ФІОТ"
  },
  {
    "id": 5143,
    "name": "УС-з51Ф",
    "faculty": "ФММ"
  }
]
```

---

## 3. Group Schedules

### 3.1 Get Group Lessons Schedule
Returns the full 2-week schedule for a specified academic group.

- **Method**: `GET`
- **URL**: `https://api.campus.kpi.ua/schedule/lessons?groupId={groupId}`
- **Parameters**: `groupId` (string / int, required) — e.g. `4402`
- **Response**: `200 OK`
```json
{
  "scheduleFirstWeek": [
    {
      "day": "Пн",
      "pairs": [
        {
          "lecturer": {
            "id": "aeb5d182368bc8faff9d186b46adca1c5b308dc80926ca96f8cf3dabe611df5c",
            "name": "Стативка Юрій Іванович"
          },
          "type": "Прак",
          "time": "18:05:00",
          "name": "Основи розробки трансляторів",
          "location": {
            "uri": "https://kpi.ua/k-5",
            "title": "5-306"
          },
          "tag": "prac",
          "dates": [
            "2026-09-07",
            "2026-09-21",
            "2026-10-05"
          ]
        }
      ]
    }
  ],
  "scheduleSecondWeek": [
    {
      "day": "Вв",
      "pairs": []
    }
  ]
}
```

---

### 3.2 Get Group Exam Schedule
- **Method**: `GET`
- **URL**: `https://api.campus.kpi.ua/schedule/exams/group?groupId={groupId}`
- **Parameters**: `groupId` (required)

---

## 4. Lecturer Schedules

### 4.1 Get Lecturer Directory
- **Method**: `GET`
- **URL**: `https://api.campus.kpi.ua/schedule/lecturer/list`

### 4.2 Get Lecturer Schedule
- **Method**: `GET`
- **URL**: `https://api.campus.kpi.ua/schedule/lecturer?lecturerId={lecturerId}`
