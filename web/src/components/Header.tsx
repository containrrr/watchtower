import logo from "../assets/logo.png"

interface HeaderProps {
    onLogOut: () => void;
}

const Header = (props: HeaderProps) => (
    <nav className="navbar shadow-sm fixed-top navbar-dark bg-secondary px-5">
        <img src={logo} className="img-fluid" alt="Watchtower logo" height="30" width="30" />

        <a className="navbar-brand mx-0" href="/">
            <span>Watchtower</span>
        </a>

        <div className="d-flex">
            <button type="button" className="btn-close btn-close-white" title="Log out" aria-label="Close" onClick={props.onLogOut}></button>
        </div>
    </nav>
);

export default Header;