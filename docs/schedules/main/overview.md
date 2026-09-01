# Main Schedule Overview (my.kpi.ua)

## 1. What is My KPI?

**My KPI** (`https://my.kpi.ua`) is the primary student and staff portal of the National Technical University of Ukraine "Igor Sikorsky Kyiv Polytechnic Institute", developed and maintained by the **Design Bureau of Information Systems (KBIS KPI)**.

It serves as the centralized digital cabinet for students, replacing older legacy systems and providing:
- Personal student profile and academic status.
- Choice of elective disciplines (вибіркові дисципліни).
- Individual curriculum and gradebook.
- **Personalized academic calendar and schedule** (`/room/student/calendar`).

---

## 2. Technical Stack of my.kpi.ua

Based on network and markup inspection:
- **Backend Framework**: **Yii2 PHP Framework** running on Apache 2.4 / Debian Linux.
- **Rendering Model**: Traditional Server-Side Rendering (SSR) with PHP view templates.
- **Frontend Assets**:
  - Bootstrap CSS & UI Theme (`/assets/.../css/bootstrap.css`, `/css/ui-theme.css`, `/css/site.css`)
  - jQuery & Yii ActiveForm scripts (`/assets/.../jquery.js`, `/assets/.../yii.js`, `/assets/.../yii.activeForm.js`)
  - jQuery UI Datepicker & Select2 for calendar and schedule widgets (`/css/print_schedule.css`, `.odd-week`, `.even-week`)
- **API Availability**: `my.kpi.ua` does not expose a public or documentation-backed REST API for external third-party developers. Access is managed through session-authenticated HTML views.

---

## 3. Why Personal Schedule Differs from Group Schedule

At KPI, higher-year curricula and modular tracks introduce significant variance among students in the **same academic group**:

1. **Selective Courses (Вибіркові дисципліни)**:
   - In semesters 3–8, students choose specialized disciplines.
   - For example, Student A may take *DevOps Technologies*, while Student B takes *Computer Graphics*.
   - The public group schedule (`schedule.kpi.ua`) displays *both* subjects in the same time slot, creating clutter.
   - `my.kpi.ua/room/student/calendar` shows *only* the specific subject enrolled by the student.

2. **Subgroups (Підгрупи)**:
   - Laboratory and practical sessions are divided into subgroup 1 and subgroup 2.
   - The public group schedule lists both pairs.
   - The personal cabinet displays the student's assigned subgroup.

Therefore, `my.kpi.ua` is the essential foundation for extracting the student's true list of classes.
