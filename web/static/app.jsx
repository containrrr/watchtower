var React;
var ReactDOM;

const useState = React.useState;
const useEffect = React.useEffect;

const api = apiFactory();

const App = () => {
    const [loading, setLoading] = useState(true);
    const [loggedIn, setLoggedIn] = useState(false);

    useEffect(() => {
        setLoading(true);
        (async () => {
            setLoggedIn(await api.checkLogin());
            setLoading(false);
        })();
    }, []);

    const onLogIn = () => {
        setLoading(false);
        setLoggedIn(true);
    }

    const onLogOut = () => {
        api.logOut();
        setLoading(false);
        setLoggedIn(false);
    };

    if (loading === true) {
        return <div class="mt-5 pt-5"><Spinner /></div>;
    }

    if (loggedIn !== true) {
        return <Login onLogin={onLogIn} />;
    }

    return (
        <content>
            <Header onLogOut={onLogOut} />
            <ContainerView api={api} />
        </content>
    );
};

const Login = (props) => {
    const [password, setPassword] = useState(undefined);
    const [remember, setRemember] = useState(false);
    const [error, setError] = useState(undefined);

    const handleChange = (event) => {
        if (event.target.type === "checkbox") {
            setRemember(event.target.checked);
        } else {
            setPassword(event.target.value);
        }
    };

    const handleSubmit = async (event) => {
        setError(undefined);
        event.preventDefault();
        const loggedIn = await api.logIn(password, remember);
        if (loggedIn) {
            props.onLogin();
        } else {
            setError("Invalid password.");
        }
    };

    return (
        <div class="d-flex flex-column min-vh-100 justify-content-center align-items-center">
            <form class="form-signin text-center" style={{ width: 350 }} onSubmit={handleSubmit}>
                <img class="mb-4" src="logo-450px.png" alt="Watchtower" width="200" height="200" />
                <h1 class="h3 mb-3 fw-normal">Please log in</h1>

                <div class="form-floating mb-3">
                    <input type="password" value={password} onChange={handleChange} class={"form-control" + (error ? " is-invalid" : "")} id="floatingPassword" placeholder="Password" required />
                    <label for="floatingPassword" class="user-select-none">Password</label>
                    {error &&
                        <div class="invalid-feedback">
                            {error}
                        </div>
                    }
                </div>
                <button class="w-100 btn btn-lg btn-primary mb-3" type="submit">Log in</button>

                <div class="checkbox mb-3">
                    <label>
                        <input type="checkbox" value="remember-me" checked={remember} onChange={handleChange} /> Remember me
                    </label>
                </div>

                <p class="mt-5 small">
                    <a href="https://containrrr.dev/watchtower/" class="text-muted small" title="Visit Watchtower" target="_blank">Powered by Watchtower</a>
                </p>
            </form>
        </div>
    );
};

const Header = (props) => (
    <nav class="navbar shadow-sm fixed-top navbar-dark bg-secondary px-5">
        <img src="logo-450px.png" class="img-fluid" alt="Watchtower logo" height="30" width="30" />

        <a class="navbar-brand mx-0" href="/">
            <span>Watchtower</span>
        </a>

        <div class="d-flex">
            <button type="button" class="btn-close btn-close-white" title="Log out" aria-label="Close" onClick={props.onLogOut}></button>
        </div>
    </nav>
);

const ContainerView = (props) => {
    const [loading, setLoading] = useState(true);
    const [checking, setChecking] = useState(false);
    const [updating, setUpdating] = useState(false);
    const [hasChecked, setHasChecked] = useState(false);
    const [viewModel, setViewModel] = useState({ Containers: [] });
    const api = props.api;

    useEffect(() => {
        listContainers();
    }, []);

    const containers = viewModel.Containers;
    const containersWithUpdates = containers.filter(c => c.HasUpdate);
    const containersWithoutUpdates = containers.filter(c => !c.HasUpdate);
    const hasSelectedContainers = containers.some(c => c.Selected);
    const hasUpdates = containersWithUpdates.length > 0;

    const checkForUpdates = async () => {
        setChecking(true);

        setViewModel(m => ({
            ...m,
            Containers: m.Containers.map(c => ({
                ...c,
                IsChecking: true
            }))
        }));

        await Promise.all(containers.map(async (c1) => {
            const result = await api.check(c1.ContainerId);
            setViewModel(m => ({
                ...m,
                Containers: m.Containers.map((c2, i) => (c1.ContainerId === c2.ContainerId ? {
                    ...c2,
                    ...result,
                    IsChecking: false
                } : c2
                ))
            }));
        }));

        setChecking(false);
        setHasChecked(true);
    };

    const listContainers = async () => {
        setLoading(true);
        const data = await api.list();
        setViewModel(data);
        setLoading(false);
        setHasChecked(false);
    };

    const updateImages = async (imagesToUpdate) => {
        setUpdating(true);
        await api.update(imagesToUpdate);
        await listContainers();
        await checkForUpdates();
        setUpdating(false);
    };

    const updateAll = async () => {
        await updateImages();
    };

    const updateSelected = async () => {
        const selectedImages = containers.filter(c => c.Selected === true).map(c => c.ImageNameShort);
        await updateImages(selectedImages);
    };

    const onContainerClick = (container) => {
        setViewModel(m => ({
            ...m,
            Containers: m.Containers.map(c2 => (container.ContainerId === c2.ContainerId ? {
                ...c2,
                Selected: !c2.Selected,
            } : c2
            ))
        }));
    };

    return (
        <main class="mt-5 p-5 d-block">
            <SpinnerModal visible={updating} title="Updating containers" message="Please wait..." />
            <div class="row mb-2">
                <div class="col-12 col-md-4 d-flex align-items-center">
                    {hasUpdates
                        ? <span>{containersWithUpdates.length} container{containersWithUpdates.length === 1 ? " has" : "s have"} updates.</span>
                        : checking
                            ? <span>Checking for updates...</span>
                            : (hasChecked && containers.length > 0)
                                ? <React.Fragment><i class="bi bi-check-circle-fill fs-4 text-primary me-2"></i><span>All containers are up to date.</span></React.Fragment>
                                : <span>{containers.length} running container{containers.length !== 1 && "s"} found.</span>}
                </div>
                <div class="col-12 col-md-8 text-end">
                    {hasUpdates && <UpdateSelected onUpdate={updateSelected} disabled={checking || !hasSelectedContainers} />}
                    {hasUpdates && <UpdateAll onUpdate={updateAll} disabled={checking} />}
                    <UpdateCheck onCheck={checkForUpdates} disabled={checking} />
                </div>
            </div>

            <ContainerList containers={containersWithUpdates} onContainerClick={onContainerClick} />

            {hasUpdates && containersWithoutUpdates.length > 0 &&
                <div class="row mt-4 mb-2">
                    <div class="col-4 d-flex align-items-center">
                        {containersWithoutUpdates.length} container{containersWithoutUpdates.length === 1 ? " is" : "s are"} up to date.
                    </div>
                </div>
            }

            <ContainerList containers={containersWithoutUpdates} onContainerClick={onContainerClick} />

            {loading && <Spinner />}
        </main>
    );
};

const UpdateSelected = (props) => (
    <button type="button" class="btn btn-primary me-2" disabled={props.disabled} onClick={props.onUpdate}>
        <i class="bi bi-arrow-down-circle me-2"></i>Update selected
    </button>
);

const UpdateAll = (props) => (
    <button type="button" class="btn btn-primary me-2" disabled={props.disabled} onClick={props.onUpdate}>
        <i class="bi bi-arrow-down-circle me-2"></i>Update all
    </button>
);

const UpdateCheck = (props) => (
    <button type="button" class="btn btn-outline-primary" disabled={props.disabled} onClick={props.onCheck}>
        <i class="bi bi-arrow-repeat me-2"></i>Check for updates
    </button>
);

const ContainerList = (props) => (
    <ul class="list-group">
        {props.containers.map(c => <ContainerListEntry {...c} onClick={() => props.onContainerClick(c)} />)}
    </ul >
);

const ContainerListEntry = (props) => (
    <li class="list-group-item d-flex justify-content-between align-items-center container-list-entry" onClick={props.onClick} role="button">
        <div class="ms-1 me-3 container-list-entry-icon">
            {props.Selected
                ? <i class="bi bi-box-fill text-primary fs-4"></i>
                : <i class="bi bi-box text-muted fs-4"></i>
            }
        </div>
        <div class="me-auto">
            <div class="fw-bold">{props.ContainerName}</div>
            <span class="user-select-all">{props.ImageName}</span> <ImageInfo version={props.ImageVersion} created={props.ImageCreatedDate} />
        </div>
        <div class="float-end d-flex align-items-center">
            {props.HasUpdate === true && <ImageInfo version={props.NewVersion} created={props.NewVersionCreated} />}
            {props.IsChecking
                ? <SpinnerGrow />
                : props.HasUpdate === true && <i class="bi bi-arrow-down-circle-fill fs-4 text-primary"></i>
            }
        </div>
    </li>
);

const ImageInfo = (props) => (
    <small class="text-muted mx-2" title={props.version + " " + props.created}>{props.created.substr(0, 10)}</small>
);

const Spinner = () => (
    <div class="d-flex justify-content-center">
        <div class="spinner-border text-muted" role="status">
            <span class="visually-hidden">Loading...</span>
        </div>
    </div>
);

const SpinnerGrow = () => (
    <div class="spinner-grow spinner-grow-sm text-primary" style={{ width: 24, height: 24 }} role="status">
        <span class="visually-hidden">Loading...</span>
    </div>
);

const SpinnerModal = (props) => {
    useEffect(() => {
        document.body.classList.toggle("modal-open", props.visible === true);
    }, [props.visible])

    return (props.visible === true &&
        <div>
            <div class="modal-backdrop fade show"></div>
            <div class="modal fade show d-block" tabindex="-1">
                <div class="modal-dialog modal-dialog-centered">
                    <div class="modal-content text-center">
                        <div class="modal-body py-5">
                            <div class="d-flex flex-column align-items-center">
                                <div class="w-50 text-center mb-4">
                                    <img src="logo-450px.png" class="img-fluid" alt="Watchtower logo" />
                                </div>

                                <h5 class="modal-title">{props.title}</h5>
                                <p class="mb-4">{props.message}</p>
                                <div class="spinner-border" role="status">
                                    <span class="visually-hidden">Loading...</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
};

const container = document.getElementById('root');
const root = ReactDOM.createRoot(container);
root.render(<App />);