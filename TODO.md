# Feature and Improvement Tracker

Doc to capture ideas for features, fixes, and friends for Donetick.

## Feature: Adding a points modal to the task detail view

I want to add a modal to quickly edit / assign points to a task, like exists for priority and history. 

## Feature: Add a points quick-select option when quick-creating a task

I want to add a quick-select option for addding points when creating a new task, like exists for description, subtasks, and due date.

## Feature: Pomodoro tracker / execution mode app wide

I want to take the execution mode that exists at the task level, and to turn this into a global execution mode to track tasks completed during a session.

Starting the global exeuction (GE) mode should suggest a list of tasks based on duration and priority. 

## Improvement/ Feature: Smarter Smart Insights

I want to see what Smart Insights is filtering for under the hood, and i want an ability to edit / add custom filters to Smart Insights.

## Feature: Reedemable rewards

Add a list of customisable urewards, and link this with the Point Redemption flow to track Points expenditure and Reward history.

The app currently supports reedeming points, see the `/circles/{id}/members/points/redeem` endpoint in  @donetick/docs/swagger.yaml. 
It even has a RedeemPointsModal defined in @donetick-frontend/src/views/Modals/RedeemPointsModal.jsx. I would like to extend this to fetch a list of reedamble rewards (according to the users circle). 

To support this, i believe we need a Rewards table. This table will be custom to each circle, and any user of that circle may edit the Rewards for that circle (unless there's any sense of circle members and owners already, in which case the owner of the circle should be the only one able to edit rewards). Rewards will have a simple name, perhaps a description, and a cost (in points). A user can redeem points for their Circle Rewards, and the Rewards a user has selected should be recorded (an extension of the current point redemption history function). 

We will also need a new Rewards page, listing only the rewards defined for the circle, which will allow members to add, remove, and edit rewards; a link to this page should be included in the sidebar under the `Points` URI.

We will also need to update the RedeemPointsModal page, and associate operations, to fetch Circle Rewards from the appropriate tables, and to display a selection of these (limited, paginated) for the user to select. This will replace the existing number selector and quick points options. All point redemption will be determined by the value of the selected rewards. 

### V2: Improved rewards history

The current functionality includes adding, editing, and removing rewards, as well as redeem points for rewards. 
The points page has a summary and analysis section showing points earned, and notes the number of points as user has redeemed. I would like to see a list of the rewards that someone has redeemed (brought up in a modal after clicking the redeemed points card). I would also like to a see a graph of showing point redemption rates, overlayed in a red line on top of points earned. 

The points page also shows the total value of all points earned in green, and redeemable points under in subtext; i would like the two flipped, so that the eye catching figure is the total number of redeemable points. Total points earned over time should still be available, but doesn't need to steal the spotlight. 

## Fix: Error parsing date during task quick-create flow

See error below:
```
{"level":"error","ts":"2026-06-14T22:09:03.248Z","caller":"chore/handler.go:383","msg":"Invalid request bodyerrorparsing time \"Mon, 15 Jun 2026 22:59:59 GMT\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"Mon, 15 Jun 2026 22:59:59 GMT\" as \"2006\"","stacktrace":"donetick.com/core/internal/chore.(*Handler).CreateChore\n\t/src/internal/chore/handler.go:383\ngithub.com/gin-gonic/gin.(*Context).Next\n\t/go/pkg/mod/github.com/gin-gonic/gin@v1.11.0/context.go:192\ndonetick.com/core/internal/chore.Routes.ImpersonationMiddleware.func2\n\t/src/internal/auth/impersonation.go:35\ngithub.com/gin-gonic/gin.(*Context).Next\n\t/go/pkg/mod/github.com/gin-gonic/gin@v1.11.0/context.go:192\ndonetick.com/core/internal/chore.Routes.(*MultiAuthMiddleware).MiddlewareFunc.func1\n\t/src/internal/auth/multiauthmiddleware.go:43\ngithub.com/gin-gonic/gin.(*Context).Next\n\t/go/pkg/mod/github.com/gin-gonic/gin@v1.11.0/context.go:192\ngithub.com/gin-gonic/gin.(*Engine).handleHTTPRequest\n\t/go/pkg/mod/github.com/gin-gonic/gin@v1.11.0/gin.go:689\ngithub.com/gin-gonic/gin.(*Engine).ServeHTTP\n\t/go/pkg/mod/github.com/gin-gonic/gin@v1.11.0/gin.go:643\nnet/http.serverHandler.ServeHTTP\n\t/usr/local/go/src/net/http/server.go:3311\nnet/http.(*conn).serve\n\t/usr/local/go/src/net/http/server.go:2073"}
```

## Feature: Day view

I want to click into a day on the calendar and see the full 24 hours (2 columns, or one long on a vertical screen) with due dates, scheduled task integrations, shifting of dates and time and moving items around a changing schedule. 

## Feature: Completed tasks

I would like to be able to see a record of completed tasks. In the home view, i would like to see the tasks completed for today below currently open tasks. 

## Feature: Hobbies

I would like to track hobbies as a seperate category of tasks, with dashboards showing completion history, streaks, trends, etc. 

## Improvement: Tab editor for tasks

Rather than scrolling down to find the right field in task edit (e.g. points) i want to see all options on a single non-scrolling screen, with different settings tabbed. 

Note, this is a particular pain point for points, and may be solved by [Feature: Adding a points modal to the task detail view].

## Improvement: Easier home navigation

Right now, i have to click on the sidebar and select all tasks in order to get home (from e.g. a task detail page). i want a single click home button (maybe a home icon in the menu bar).